package fetcher

import (
	"fmt"
	"io"
	"log"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/internal/providers"
)

//go:generate counterfeiter -o ./fakes/s3_provider.go --fake-name S3Provider . s3Provider
type s3Provider interface {
	GetS3Downloader(region, accessKeyID, secretAccessKey string) providers.S3Downloader
	GetS3Client(region, accessKeyID, secretAccessKey string) s3iface.S3API
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
