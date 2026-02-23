package component

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/semver/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

//counterfeiter:generate -o ./fakes/s3_downloader.go --fake-name S3Downloader . S3Downloader
type S3Downloader interface {
	DownloadObject(ctx context.Context, input *transfermanager.DownloadObjectInput, opts ...func(*transfermanager.Options)) (*transfermanager.DownloadObjectOutput, error)
}

//counterfeiter:generate -o ./fakes/s3_client.go --fake-name S3Client . S3Client
type S3Client interface {
	HeadObject(ctx context.Context, input *s3.HeadObjectInput, options ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, options ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type S3ReleaseSource struct {
	cargo.ReleaseSourceConfig

	s3Client     S3Client
	s3Downloader S3Downloader

	DownloadThreads int

	logger *log.Logger
}

func NewS3ReleaseSource(rsConfig cargo.ReleaseSourceConfig, client S3Client, downloader S3Downloader, logger *log.Logger) S3ReleaseSource {
	if rsConfig.Type != "" && rsConfig.Type != ReleaseSourceTypeS3 {
		panic(panicMessageWrongReleaseSourceType)
	}

	if logger == nil {
		logger = log.New(os.Stderr, "[S3 release source] ", log.Default().Flags())
	}

	return S3ReleaseSource{
		ReleaseSourceConfig: rsConfig,
		s3Client:            client,
		s3Downloader:        downloader,
		logger:              logger,
	}
}

func NewS3ReleaseSourceFromConfig(rsConfig cargo.ReleaseSourceConfig, logger *log.Logger) S3ReleaseSource {
	validateConfig(rsConfig)

	awsConfig, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(rsConfig.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(rsConfig.AccessKeyId, rsConfig.SecretAccessKey, "")),
	)
	if err != nil {
		// TODO: add test coverage for this block
		panic(err)
	}

	if rsConfig.RoleARN != "" {
		// TODO: add test coverage for this block
		stsClient := sts.NewFromConfig(awsConfig)

		awsConfig, err = config.LoadDefaultConfig(
			context.Background(),
			config.WithRegion(rsConfig.Region),
			config.WithCredentialsProvider(stscreds.NewAssumeRoleProvider(stsClient, rsConfig.RoleARN)),
		)
		if err != nil {
			// TODO: add test coverage for this block
			panic(err)
		}
	}

	client := s3.NewFromConfig(awsConfig)

	return NewS3ReleaseSource(
		rsConfig,
		client,
		transfermanager.New(client),
		logger,
	)
}

func validateConfig(rsConfig cargo.ReleaseSourceConfig) {
	if rsConfig.PathTemplate == "" {
		panic(`Missing required field "path_template" in release source config. Is your Kilnfile out of date?`)
	}
	if rsConfig.Bucket == "" {
		panic(`Missing required field "bucket" in release source rsConfig. Is your Kilnfile out of date?`)
	}
}

func (src S3ReleaseSource) ID() string                               { return src.ReleaseSourceConfig.ID }
func (src S3ReleaseSource) Publishable() bool                        { return src.ReleaseSourceConfig.Publishable }
func (src S3ReleaseSource) Configuration() cargo.ReleaseSourceConfig { return src.ReleaseSourceConfig }

func (src S3ReleaseSource) GetMatchedRelease(spec cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	_, err = src.s3Client.HeadObject(context.Background(),
		&s3.HeadObjectInput{
			Bucket: aws.String(src.Bucket),
			Key:    aws.String(remotePath),
		})
	if err != nil {
		var nfErr *s3types.NotFound

		if errors.As(err, &nfErr) {
			return cargo.BOSHReleaseTarballLock{}, ErrNotFound
		}
		return cargo.BOSHReleaseTarballLock{}, err
	}

	return cargo.BOSHReleaseTarballLock{
		Name:         spec.Name,
		Version:      spec.Version,
		RemotePath:   remotePath,
		RemoteSource: src.ID(),
	}, nil
}

func (src S3ReleaseSource) FindReleaseVersion(spec cargo.BOSHReleaseTarballSpecification, noDownload bool) (cargo.BOSHReleaseTarballLock, error) {
	pathTemplatePattern, _ := regexp.Compile(`^\d+\.\d+`)
	tasVersion := pathTemplatePattern.FindString(src.PathTemplate)
	var prefix string
	if tasVersion != "" {
		prefix = tasVersion + "/"
	}
	prefix += spec.Name + "/"

	releaseResults, err := src.s3Client.ListObjectsV2(context.Background(),
		&s3.ListObjectsV2Input{
			Bucket: &src.Bucket,
			Prefix: &prefix,
		})
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	semverPattern, err := regexp.Compile(`([-v])\d+(.\d+)*`)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	foundRelease := cargo.BOSHReleaseTarballLock{}
	constraint, err := spec.VersionConstraints()
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	for _, result := range releaseResults.Contents {
		versions := semverPattern.FindAllString(*result.Key, -1)
		version := versions[0]
		stemcellVersion := versions[len(versions)-1]
		version = strings.ReplaceAll(version, "-", "")
		version = strings.ReplaceAll(version, "v", "")
		stemcellVersion = strings.ReplaceAll(stemcellVersion, "-", "")
		if len(versions) > 1 && stemcellVersion != spec.StemcellVersion {
			continue
		}
		if version != "" {
			newVersion, _ := semver.NewVersion(version)
			if !constraint.Check(newVersion) {
				continue
			}

			if (foundRelease == cargo.BOSHReleaseTarballLock{}) {
				foundRelease = cargo.BOSHReleaseTarballLock{
					Name:         spec.Name,
					Version:      version,
					RemotePath:   *result.Key,
					RemoteSource: src.ReleaseSourceConfig.ID,
				}
			} else {
				foundVersion, _ := semver.NewVersion(foundRelease.Version)
				if newVersion.GreaterThan(foundVersion) {
					foundRelease = cargo.BOSHReleaseTarballLock{
						Name:         spec.Name,
						Version:      version,
						RemotePath:   *result.Key,
						RemoteSource: src.ReleaseSourceConfig.ID,
					}
				}
			}
		}
	}
	if (foundRelease == cargo.BOSHReleaseTarballLock{}) {
		return cargo.BOSHReleaseTarballLock{}, ErrNotFound
	}

	if noDownload {
		foundRelease.SHA1 = "not-calculated"
	} else {
		var releaseLocal Local
		releaseLocal, err = src.DownloadRelease("/tmp", foundRelease)
		if err != nil {
			return cargo.BOSHReleaseTarballLock{}, err
		}
		foundRelease.SHA1 = releaseLocal.Lock.SHA1
	}
	return foundRelease, nil
}

func (src S3ReleaseSource) DownloadRelease(releaseDir string, lock cargo.BOSHReleaseTarballLock) (Local, error) {
	setConcurrency := func(opts *transfermanager.Options) {
		if src.DownloadThreads > 0 {
			opts.Concurrency = src.DownloadThreads
		}
	}

	src.logger.Printf(logLineDownload, lock.Name, lock.Version, ReleaseSourceTypeS3, src.ID())

	outputFile := filepath.Join(releaseDir, filepath.Base(lock.RemotePath))

	file, err := os.Create(outputFile)
	if err != nil {
		return Local{}, fmt.Errorf("failed to create file %q: %w", outputFile, err)
	}
	defer closeAndIgnoreError(file)

	_, err = src.s3Downloader.DownloadObject(context.Background(),
		&transfermanager.DownloadObjectInput{
			Bucket:   aws.String(src.Bucket),
			Key:      aws.String(lock.RemotePath),
			WriterAt: file,
		}, setConcurrency)
	if err != nil {
		return Local{}, fmt.Errorf("failed to download file: %w", err)
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

func (src S3ReleaseSource) RemotePath(spec cargo.BOSHReleaseTarballSpecification) (string, error) {
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
			Parse(src.PathTemplate))
}
