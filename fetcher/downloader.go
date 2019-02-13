package fetcher

import (
	"fmt"
	"io"
	"log"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

//go:generate counterfeiter -o ./fakes/s3_downloader.go --fake-name S3Downloader . S3Downloader
type S3Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

//go:generate counterfeiter -o ./fakes/s3_provider.go --fake-name S3Provider . s3Provider
type s3Provider interface {
	GetS3Downloader(region, accessKeyID, secretAccessKey string) S3Downloader
	GetS3Client(region, accessKeyID, secretAccessKey string) s3iface.S3API
}

// TODO: move this implementation
type S3Provider struct {
}

func NewS3Provider() S3Provider {
	return S3Provider{}
}

func (s S3Provider) GetS3Downloader(region, accessKeyID, secretAccessKey string) S3Downloader {
	return s3manager.NewDownloaderWithClient(s.GetS3Client(region, accessKeyID, secretAccessKey))
}

func (s S3Provider) GetS3Client(region, accessKeyID, secretAccessKey string) s3iface.S3API {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
	}))
	return s3.New(sess)
}

type Downloader struct {
	logger      *log.Logger
	s3Provider  s3Provider
	fileCreator func(string) (io.WriterAt, error)
}

func NewDownloader(logger *log.Logger, s3Provider s3Provider, fileCreator func(string) (io.WriterAt, error)) Downloader {
	return Downloader{
		logger:      logger,
		s3Provider:  s3Provider,
		fileCreator: fileCreator,
	}
}

func (d Downloader) DownloadReleases(releaseDir string, compiledReleases cargo.CompiledReleases, matchedS3Objects map[cargo.CompiledRelease]string, downloadThreads int) error {
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	s3Downloader := d.s3Provider.GetS3Downloader(compiledReleases.Region, compiledReleases.AccessKeyId, compiledReleases.SecretAccessKey)

	for release, path := range matchedS3Objects {
		outputFile := fmt.Sprintf("%s-%s-%s-%s.tgz", release.Name, release.Version, release.StemcellOS, release.StemcellVersion)
		file, err := d.fileCreator(filepath.Join(releaseDir, outputFile))
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", outputFile, err)
		}

		d.logger.Printf("downloading %s...\n", path)
		_, err = s3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(compiledReleases.Bucket),
			Key:    aws.String(path),
		}, setConcurrency)

		if err != nil {
			return fmt.Errorf("failed to download file, %v\n", err)
		}
	}

	return nil
}
