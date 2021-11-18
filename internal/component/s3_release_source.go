package component

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/semver"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

//counterfeiter:generate -o ./fakes/s3_downloader.go --fake-name S3Downloader . S3Downloader
type S3Downloader interface {
	DownloadWithContext(ctx aws.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

//counterfeiter:generate -o ./fakes/s3_uploader.go --fake-name S3Uploader . S3Uploader
type S3Uploader interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

//counterfeiter:generate -o ./fakes/s3_client.go --fake-name S3Client . S3Client
type S3Client interface {
	HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
}

type S3ReleaseSource struct {
	cargo.ReleaseSourceConfig

	s3Client     S3Client
	s3Downloader S3Downloader
	s3Uploader   S3Uploader

	DownloadThreads int

	logger *log.Logger
}

func NewS3ReleaseSource(c cargo.ReleaseSourceConfig, client S3Client, downloader S3Downloader, uploader S3Uploader, logger *log.Logger) S3ReleaseSource {
	if c.Type != "" && c.Type != ReleaseSourceTypeS3 {
		panic(panicMessageWrongReleaseSourceType)
	}
	return S3ReleaseSource{
		ReleaseSourceConfig: c,
		s3Client:            client,
		s3Downloader:        downloader,
		s3Uploader:          uploader,
		logger:              logger,
	}
}

func NewS3ReleaseSourceFromConfig(config cargo.ReleaseSourceConfig, logger *log.Logger) S3ReleaseSource {
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
		config,
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

func (src S3ReleaseSource) ID() string                               { return src.ReleaseSourceConfig.ID }
func (src S3ReleaseSource) Publishable() bool                        { return src.ReleaseSourceConfig.Publishable }
func (src S3ReleaseSource) Configuration() cargo.ReleaseSourceConfig { return src.ReleaseSourceConfig }

//counterfeiter:generate -o ./fakes/s3_request_failure.go --fake-name S3RequestFailure github.com/aws/aws-sdk-go/service/s3.RequestFailure
func (src S3ReleaseSource) GetMatchedRelease(spec Spec) (Lock, bool, error) {
	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return Lock{}, false, err
	}

	headRequest := new(s3.HeadObjectInput)
	headRequest.SetBucket(src.ReleaseSourceConfig.Bucket)
	headRequest.SetKey(remotePath)

	_, err = src.s3Client.HeadObject(headRequest)
	if err != nil {
		requestFailure, ok := err.(s3.RequestFailure)
		if ok && requestFailure.StatusCode() == 404 {
			return Lock{}, false, nil
		}
		return Lock{}, false, err
	}

	return Lock{
		Name:         spec.Name,
		Version:      spec.Version,
		RemotePath:   remotePath,
		RemoteSource: src.ID(),
	}, true, nil
}

func (src S3ReleaseSource) FindReleaseVersion(spec Spec) (Lock, bool, error) {
	pathTemplatePattern, _ := regexp.Compile(`^\d+\.\d+`)
	tasVersion := pathTemplatePattern.FindString(src.ReleaseSourceConfig.PathTemplate)
	var prefix string
	if tasVersion != "" {
		prefix = tasVersion + "/"
	}
	prefix += spec.Name + "/"

	releaseResults, err := src.s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: &src.ReleaseSourceConfig.Bucket,
		Prefix: &prefix,
	})
	if err != nil {
		return Lock{}, false, err
	}

	semverPattern, err := regexp.Compile(`([-v])\d+(.\d+)*`)
	if err != nil {
		return Lock{}, false, err
	}

	foundRelease := Lock{}
	constraint, err := spec.VersionConstraints()
	if err != nil {
		return Lock{}, false, err
	}

	for _, result := range releaseResults.Contents {
		versions := semverPattern.FindAllString(*result.Key, -1)
		version := versions[0]
		stemcellVersion := versions[len(versions)-1]
		version = strings.Replace(version, "-", "", -1)
		version = strings.Replace(version, "v", "", -1)
		stemcellVersion = strings.Replace(stemcellVersion, "-", "", -1)
		if len(versions) > 1 && stemcellVersion != spec.StemcellVersion {
			continue
		}
		if version != "" {
			newVersion, _ := semver.NewVersion(version)
			if !constraint.Check(newVersion) {
				continue
			}

			if (foundRelease == Lock{}) {
				foundRelease = Lock{
					Name:         spec.Name,
					Version:      version,
					RemotePath:   *result.Key,
					RemoteSource: src.ReleaseSourceConfig.ID,
				}
			} else {
				foundVersion, _ := semver.NewVersion(foundRelease.Version)
				if newVersion.GreaterThan(foundVersion) {
					foundRelease = Lock{
						Name:         spec.Name,
						Version:      version,
						RemotePath:   *result.Key,
						RemoteSource: src.ReleaseSourceConfig.ID,
					}
				}
			}
		}
	}
	if (foundRelease == Lock{}) {
		return Lock{}, false, nil
	}
	hash := sha1.New()
	err = src.DownloadComponent(context.TODO(), hash, foundRelease)
	if err != nil {
		return Lock{}, false, err
	}
	foundRelease.SHA1 = hex.EncodeToString(hash.Sum(nil))
	return foundRelease, true, nil
}

// TODO add logger for all release sources: src.logger.Printf("downloading %s %s from %s", lock.Name, lock.Version, src.ReleaseSourceConfig.Bucket)

func (src S3ReleaseSource) DownloadComponent(ctx context.Context, w io.Writer, remoteRelease Lock) error {
	setConcurrency := func(dl *s3manager.Downloader) {
		if src.DownloadThreads > 0 {
			dl.Concurrency = src.DownloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	temporaryFile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(temporaryFile.Name())
	}()

	_, err = src.s3Downloader.DownloadWithContext(ctx, temporaryFile, &s3.GetObjectInput{
		Bucket: aws.String(src.ReleaseSourceConfig.Bucket),
		Key:    aws.String(remoteRelease.RemotePath),
	}, setConcurrency)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, temporaryFile)

	return err
}

func (src S3ReleaseSource) UploadRelease(spec Spec, file io.Reader) (Lock, error) {
	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	src.logger.Printf("uploading release %q to %s at %q...\n", spec.Name, src.ID(), remotePath)

	_, err = src.s3Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(src.ReleaseSourceConfig.Bucket),
		Key:    aws.String(remotePath),
		Body:   file,
	})
	if err != nil {
		return Lock{}, err
	}

	return Lock{
		Name:         spec.Name,
		Version:      spec.Version,
		RemotePath:   remotePath,
		RemoteSource: src.ID(),
	}, nil
}

func (src S3ReleaseSource) RemotePath(spec Spec) (string, error) {
	pathBuf := new(bytes.Buffer)

	err := src.pathTemplate().Execute(pathBuf, spec)
	if err != nil {
		return "", fmt.Errorf("unable to evaluate path_template: %w", err)
	}

	return pathBuf.String(), nil
}

func (src S3ReleaseSource) pathTemplate() *template.Template {
	return template.Must(
		template.New("remote-path").
			Funcs(template.FuncMap{"trimSuffix": strings.TrimSuffix}).
			Parse(src.ReleaseSourceConfig.PathTemplate))
}
