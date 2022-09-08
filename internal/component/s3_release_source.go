package component

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

//counterfeiter:generate -o ./fakes/s3_downloader.go --fake-name S3Downloader . S3Downloader
type S3Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
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
	Identifier  string `yaml:"id,omitempty"`
	Publishable bool   `yaml:"publishable,omitempty"`

	Endpoint        string `yaml:"endpoint,omitempty"`
	Bucket          string `yaml:"bucket,omitempty"`
	Region          string `yaml:"region,omitempty"`
	AccessKeyId     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	PathTemplate    string `yaml:"path_template,omitempty"`

	DownloadThreads int `yaml:"-"`

	Collaborators struct {
		InitOnce sync.Once
		S3Client
		S3Downloader
		S3Uploader
	}
}

func (src *S3ReleaseSource) ConfigurationErrors() []error {
	var result []error
	if src.PathTemplate == "" {
		result = append(result, fmt.Errorf(`missing required field "path_template" in release source config`))
	}
	if src.Bucket == "" {
		result = append(result, fmt.Errorf(`missing required field "bucket" in release source config`))
	}
	return result
}

func (src *S3ReleaseSource) ID() string {
	if src.Identifier != "" {
		return src.Identifier
	}
	return src.Bucket
}

func (src *S3ReleaseSource) IsPublishable() bool { return src.Publishable }
func (src *S3ReleaseSource) Type() string        { return ReleaseSourceTypeS3 }

func (src *S3ReleaseSource) init() error {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/

	var initErr error
	src.Collaborators.InitOnce.Do(func() {
		awsConfig := &aws.Config{
			Region:      aws.String(src.Region),
			Credentials: credentials.NewStaticCredentials(src.AccessKeyId, src.SecretAccessKey, ""),
		}
		var sess *session.Session

		sess, initErr = session.NewSession(awsConfig)
		if initErr != nil {
			return
		}

		client := s3.New(sess)
		if src.Endpoint != "" { // for acceptance testing
			awsConfig = awsConfig.WithEndpoint(src.Endpoint)
			awsConfig = awsConfig.WithS3ForcePathStyle(true)
		}

		src.Collaborators.S3Client = client
		src.Collaborators.S3Downloader = s3manager.NewDownloaderWithClient(client)
		src.Collaborators.S3Uploader = s3manager.NewUploaderWithClient(client)
	})
	return initErr
}

//counterfeiter:generate -o ./fakes/s3_request_failure.go --fake-name S3RequestFailure github.com/aws/aws-sdk-go/service/s3.RequestFailure
func (src *S3ReleaseSource) GetMatchedRelease(_ context.Context, _ *log.Logger, spec Spec) (Lock, error) {
	err := src.init()
	if err != nil {
		return Lock{}, err
	}

	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	headRequest := new(s3.HeadObjectInput)
	headRequest.SetBucket(src.Bucket)
	headRequest.SetKey(remotePath)

	_, err = src.Collaborators.S3Client.HeadObject(headRequest)
	if err != nil {
		requestFailure, ok := err.(s3.RequestFailure)
		if ok && requestFailure.StatusCode() == 404 {
			return Lock{}, ErrNotFound
		}
		return Lock{}, err
	}

	return Lock{
		Name:         spec.Name,
		Version:      spec.Version,
		RemotePath:   remotePath,
		RemoteSource: src.ID(),
	}, nil
}

func (src *S3ReleaseSource) FindReleaseVersion(ctx context.Context, logger *log.Logger, spec Spec) (Lock, error) {
	err := src.init()
	if err != nil {
		return Lock{}, err
	}

	pathTemplatePattern, _ := regexp.Compile(`^\d+\.\d+`)
	tasVersion := pathTemplatePattern.FindString(src.PathTemplate)
	var prefix string
	if tasVersion != "" {
		prefix = tasVersion + "/"
	}
	prefix += spec.Name + "/"

	releaseResults, err := src.Collaborators.S3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: &src.Bucket,
		Prefix: &prefix,
	})
	if err != nil {
		return Lock{}, err
	}

	semverPattern, err := regexp.Compile(`([-v])\d+(.\d+)*`)
	if err != nil {
		return Lock{}, err
	}

	foundRelease := Lock{}
	constraint, err := spec.VersionConstraints()
	if err != nil {
		return Lock{}, err
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
					RemoteSource: src.ID(),
				}
			} else {
				foundVersion, _ := semver.NewVersion(foundRelease.Version)
				if newVersion.GreaterThan(foundVersion) {
					foundRelease = Lock{
						Name:         spec.Name,
						Version:      version,
						RemotePath:   *result.Key,
						RemoteSource: src.ID(),
					}
				}
			}
		}
	}
	if (foundRelease == Lock{}) {
		return Lock{}, ErrNotFound
	}
	var releaseLocal Local
	releaseLocal, err = src.DownloadRelease(ctx, logger, "/tmp", foundRelease)
	if err != nil {
		return Lock{}, err
	}
	foundRelease.SHA1 = releaseLocal.SHA1
	return foundRelease, nil
}

func (src *S3ReleaseSource) DownloadRelease(_ context.Context, logger *log.Logger, releaseDir string, lock Lock) (Local, error) {
	err := src.init()
	if err != nil {
		return Local{}, err
	}

	setConcurrency := func(dl *s3manager.Downloader) {
		if src.DownloadThreads > 0 {
			dl.Concurrency = src.DownloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	logger.Printf(logLineDownload, lock.Name, ReleaseSourceTypeS3, src.ID())

	outputFile := filepath.Join(releaseDir, filepath.Base(lock.RemotePath))

	file, err := os.Create(outputFile)
	if err != nil {
		return Local{}, fmt.Errorf("failed to create file %q: %w", outputFile, err)
	}
	defer closeAndIgnoreError(file)

	_, err = src.Collaborators.S3Downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(src.Bucket),
		Key:    aws.String(lock.RemotePath),
	}, setConcurrency)
	if err != nil {
		return Local{}, fmt.Errorf("failed to download file: %w\n", err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return Local{}, fmt.Errorf("error reseting file cursor: %w", err) // untested
	}

	hash := sha1.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return Local{}, fmt.Errorf("error hashing file contents: %w", err) // untested
	}

	lock.SHA1 = hex.EncodeToString(hash.Sum(nil))

	return Local{Lock: lock, LocalPath: outputFile}, nil
}

func (src *S3ReleaseSource) UploadRelease(_ context.Context, logger *log.Logger, spec Spec, file io.Reader) (Lock, error) {
	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	logger.Printf("uploading release %q to %s at %q...\n", spec.Name, src.Bucket, remotePath)

	_, err = src.Collaborators.S3Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(src.Bucket),
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
		RemoteSource: src.Bucket,
	}, nil
}

func (src *S3ReleaseSource) RemotePath(spec Spec) (string, error) {
	pathBuf := new(bytes.Buffer)

	err := src.pathTemplate().Execute(pathBuf, spec)
	if err != nil {
		return "", fmt.Errorf("unable to evaluate path_template: %w", err)
	}

	return pathBuf.String(), nil
}

func (src *S3ReleaseSource) pathTemplate() *template.Template {
	return template.Must(
		template.New("remote-path").
			Funcs(template.FuncMap{"trimSuffix": strings.TrimSuffix}).
			Parse(src.PathTemplate))
}
