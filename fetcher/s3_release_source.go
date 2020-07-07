package fetcher

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/Masterminds/semver"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/pivotal-cf/kiln/release"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

//go:generate counterfeiter -o ./fakes/s3_downloader.go --fake-name S3Downloader . S3Downloader
type S3Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

//go:generate counterfeiter -o ./fakes/s3_uploader.go --fake-name S3Uploader . S3Uploader
type S3Uploader interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

//go:generate counterfeiter -o ./fakes/s3_client.go --fake-name S3Client . S3Client
type S3Client interface {
	HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
}

type S3ReleaseSource struct {
	id                 string
	bucket             string
	pathTemplateString string
	publishable        bool

	s3Client     S3Client
	s3Downloader S3Downloader
	s3Uploader   S3Uploader

	logger *log.Logger
}

func NewS3ReleaseSource(id, bucket, pathTemplate string, publishable bool, client S3Client, downloader S3Downloader, uploader S3Uploader, logger *log.Logger) S3ReleaseSource {
	return S3ReleaseSource{
		id:                 id,
		bucket:             bucket,
		pathTemplateString: pathTemplate,
		publishable:        publishable,
		s3Client:           client,
		s3Downloader:       downloader,
		s3Uploader:         uploader,
		logger:             logger,
	}
}

func S3ReleaseSourceFromConfig(config cargo.ReleaseSourceConfig, logger *log.Logger) S3ReleaseSource {
	validateConfig(config)

	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	awsConfig := &aws.Config{
		Region:      aws.String(config.Region),
		Credentials: credentials.NewStaticCredentials(config.AccessKeyId, config.SecretAccessKey, ""),
	}
	if config.Endpoint != "" { // for acceptance testing
		awsConfig = awsConfig.WithEndpoint(config.Endpoint)
		awsConfig = awsConfig.WithS3ForcePathStyle(true)
	}

	sess := session.Must(session.NewSession(awsConfig))
	client := s3.New(sess)

	return NewS3ReleaseSource(
		config.ID,
		config.Bucket,
		config.PathTemplate,
		config.Publishable,
		client,
		s3manager.NewDownloaderWithClient(client),
		s3manager.NewUploaderWithClient(client),
		logger,
	)
}

func validateConfig(config cargo.ReleaseSourceConfig) {
	if config.PathTemplate == "" {
		panic(`Missing required field "path_template" in release source config. Is your Kilnfile out of date?`)
	}
	if config.Bucket == "" {
		panic(`Missing required field "bucket" in release source config. Is your Kilnfile out of date?`)
	}
}

func (src S3ReleaseSource) ID() string {
	return src.id
}

func (src S3ReleaseSource) Publishable() bool {
	return src.publishable
}

//go:generate counterfeiter -o ./fakes/s3_request_failure.go --fake-name S3RequestFailure github.com/aws/aws-sdk-go/service/s3.RequestFailure
func (src S3ReleaseSource) GetMatchedRelease(requirement release.Requirement) (release.Remote, bool, error) {
	remotePath, err := src.RemotePath(requirement)
	if err != nil {
		return release.Remote{}, false, err
	}

	headRequest := new(s3.HeadObjectInput)
	headRequest.SetBucket(src.bucket)
	headRequest.SetKey(remotePath)

	_, err = src.s3Client.HeadObject(headRequest)
	if err != nil {
		requestFailure, ok := err.(s3.RequestFailure)
		if ok && requestFailure.StatusCode() == 404 {
			return release.Remote{}, false, nil
		}
		return release.Remote{}, false, err
	}

	return release.Remote{
		ID:         release.ID{Name: requirement.Name, Version: requirement.Version},
		RemotePath: remotePath,
		SourceID:   src.ID(),
	}, true, nil
}

type ByVersion []release.Remote

func (a ByVersion) Len() int      { return len(a) }
func (a ByVersion) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByVersion) Less(i, j int) bool {
	v1, _ := semver.NewVersion(a[i].ID.Version)
	v2, _ := semver.NewVersion(a[j].ID.Version)
	if v1.Equal(v2) {
		// we have two files with the same version, so compare the stemcell versions
		s1 := getStemcellVersion(a[i].RemotePath)
		s2 := getStemcellVersion(a[j].RemotePath)
		return s1.LessThan(s2)
	}
	return v1.LessThan(v2)
}

func getStemcellVersion(path string) *semver.Version {
	stemcellVersion, _ := regexp.Compile(`-(\d+\.\d+).tgz`)
	matchedVersions := stemcellVersion.FindStringSubmatch(path)
	version := "0.0.0"
	if matchedVersions != nil {
		version = matchedVersions[1]
	}
	retVersion, _ := semver.NewVersion(version)
	return retVersion
}

func (src S3ReleaseSource) FindReleaseVersion(requirement release.Requirement) (release.Remote, bool, error) {
	pathTemplatePattern, _ := regexp.Compile(`^\d+\.\d+`)
	tasVersion := pathTemplatePattern.FindString(src.pathTemplateString)
	var prefix string
	if tasVersion != "" {
		prefix = tasVersion + "/"
	}
	prefix += requirement.Name + "/"

	releaseResults, err := src.s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: &src.bucket,
		Prefix: &prefix,
	})
	if err != nil {
		return release.Remote{}, false, err
	}

	semverPattern, err := regexp.Compile("-\\d+(.\\d+)*")
	if err != nil {
		return release.Remote{}, false, err
	}

	foundRelease := release.Remote{}
	var constraint *semver.Constraints
	if requirement.VersionConstraint != "" {
		constraint, _ = semver.NewConstraint(requirement.VersionConstraint)
	} else {
		constraint, _ = semver.NewConstraint(">0")
	}
	for _, result := range releaseResults.Contents {
		versions := semverPattern.FindAllString(*result.Key, -1)
		version := versions[0]
		stemcellVersion := versions[len(versions)-1]
		version = strings.Replace(version, "-", "", -1)
		stemcellVersion = strings.Replace(stemcellVersion, "-", "", -1)
		if len(versions) > 1 && stemcellVersion != requirement.StemcellVersion {
			continue
		}
		if version != "" {
			newVersion, _ := semver.NewVersion(version)
			if !constraint.Check(newVersion) {
				continue
			}

			if (foundRelease == release.Remote{}) {
				foundRelease = release.Remote{
					ID: release.ID{
						Name:    requirement.Name,
						Version: version,
					},
					RemotePath: *result.Key,
					SourceID:   src.id,
				}
			} else {
				foundVersion, _ := semver.NewVersion(foundRelease.Version)
				if newVersion.GreaterThan(foundVersion) {
					foundRelease = release.Remote{
						ID: release.ID{
							Name:    requirement.Name,
							Version: version,
						},
						RemotePath: *result.Key,
						SourceID:   src.id,
					}
				}
			}
		}
	}
	if (foundRelease == release.Remote{}) {
		return release.Remote{}, false, nil
	}
	var releaseLocal release.Local
	releaseLocal, err = src.DownloadRelease("/tmp", foundRelease, DefaultDownloadThreadCount)
	if err != nil {
		return release.Remote{}, false, err
	}
	foundRelease.SHA = releaseLocal.SHA1
	return foundRelease, true, nil
}

func (src S3ReleaseSource) DownloadRelease(releaseDir string, remoteRelease release.Remote, downloadThreads int) (release.Local, error) {
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	src.logger.Printf("downloading %s %s from %s", remoteRelease.Name, remoteRelease.Version, src.bucket)

	outputFile := filepath.Join(releaseDir, filepath.Base(remoteRelease.RemotePath))

	file, err := os.Create(outputFile)
	if err != nil {
		return release.Local{}, fmt.Errorf("failed to create file %q: %w", outputFile, err)
	}
	defer file.Close()

	_, err = src.s3Downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(src.bucket),
		Key:    aws.String(remoteRelease.RemotePath),
	}, setConcurrency)
	if err != nil {
		return release.Local{}, fmt.Errorf("failed to download file: %w\n", err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return release.Local{}, fmt.Errorf("error reseting file cursor: %w", err) // untested
	}

	hash := sha1.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return release.Local{}, fmt.Errorf("error hashing file contents: %w", err) // untested
	}

	sha1 := hex.EncodeToString(hash.Sum(nil))

	return release.Local{ID: remoteRelease.ID, LocalPath: outputFile, SHA1: sha1}, nil
}

func (src S3ReleaseSource) UploadRelease(spec release.Requirement, file io.Reader) (release.Remote, error) {
	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return release.Remote{}, err
	}

	src.logger.Printf("uploading release %q to %s at %q...\n", spec.Name, src.ID(), remotePath)

	_, err = src.s3Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(src.bucket),
		Key:    aws.String(remotePath),
		Body:   file,
	})
	if err != nil {
		return release.Remote{}, err
	}

	return release.Remote{
		ID:         release.ID{Name: spec.Name, Version: spec.Version},
		RemotePath: remotePath,
		SourceID:   src.ID(),
	}, nil
}

func (src S3ReleaseSource) RemotePath(requirement release.Requirement) (string, error) {
	pathBuf := new(bytes.Buffer)

	err := src.pathTemplate().Execute(pathBuf, requirement)
	if err != nil {
		return "", fmt.Errorf("unable to evaluate path_template: %w", err)
	}

	return pathBuf.String(), nil
}

func (src S3ReleaseSource) pathTemplate() *template.Template {
	return template.Must(
		template.New("remote-path").
			Funcs(template.FuncMap{"trimSuffix": strings.TrimSuffix}).
			Parse(src.pathTemplateString))
}
