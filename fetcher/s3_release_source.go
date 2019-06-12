package fetcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/internal/providers"
)

type S3Connecter struct {
	logger     *log.Logger
	s3Provider S3Provider
}

func NewS3Connecter(s3Provider S3Provider, logger *log.Logger) S3Connecter {
	return S3Connecter{
		logger:     logger,
		s3Provider: s3Provider,
	}
}

func (c S3Connecter) Connect(compiledReleases cargo.CompiledReleases) commands.ReleaseSource {
	s3Client := c.s3Provider.GetS3Client(compiledReleases.Region, compiledReleases.AccessKeyId, compiledReleases.SecretAccessKey)
	s3Downloader := c.s3Provider.GetS3Downloader(compiledReleases.Region, compiledReleases.AccessKeyId, compiledReleases.SecretAccessKey)

	return S3ReleaseSource{
		logger:       c.logger,
		s3Client:     s3Client,
		s3Downloader: s3Downloader,
	}
}

type S3ReleaseSource struct {
	logger       *log.Logger
	s3Provider   S3Provider //<--- No More (remove later)
	s3Client     s3iface.S3API
	s3Downloader providers.S3Downloader
}

//go:generate counterfeiter -o ./fakes/s3_provider.go --fake-name S3Provider . S3Provider
type S3Provider interface {
	GetS3Downloader(region, accessKeyID, secretAccessKey string) providers.S3Downloader
	GetS3Client(region, accessKeyID, secretAccessKey string) s3iface.S3API
}

//go:generate counterfeiter -o ./fakes/s3client.go --fake-name S3Thingie github.com/pivotal-cf/kiln/vendor/github.com/aws/aws-sdk-go/service/s3/s3iface.S3API
func (r S3ReleaseSource) GetMatchedReleases(compiledReleases cargo.CompiledReleases, assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, []cargo.CompiledRelease, error) {
	matchedS3Objects := make(map[cargo.CompiledRelease]string)

	regex, err := NewCompiledReleasesRegexp(compiledReleases.Regex)
	if err != nil {
		return nil, nil, err
	}

	s3Client := r.s3Client

	err = s3Client.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: aws.String(compiledReleases.Bucket),
		},
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			for _, s3Object := range page.Contents {
				if s3Object.Key == nil {
					continue
				}

				compiledRelease, err := regex.Convert(*s3Object.Key)
				if err != nil {
					continue
				}

				matchedS3Objects[compiledRelease] = *s3Object.Key
			}
			return true
		},
	)
	if err != nil {
		return nil, nil, err
	}

	missingReleases := make([]cargo.CompiledRelease, 0)
	matchingReleases := make(map[cargo.CompiledRelease]string, 0)
	for _, release := range assetsLock.Releases {
		expectedRelease := cargo.CompiledRelease{
			Name:            release.Name,
			Version:         release.Version,
			StemcellOS:      assetsLock.Stemcell.OS,
			StemcellVersion: assetsLock.Stemcell.Version,
		}
		s3Key, ok := matchedS3Objects[expectedRelease]

		if !ok {
			missingReleases = append(missingReleases, expectedRelease)
		} else {
			matchingReleases[expectedRelease] = s3Key
		}
	}

	return matchingReleases, missingReleases, nil
}

func (r S3ReleaseSource) DownloadReleases(releaseDir string, compiledReleases cargo.CompiledReleases, matchedS3Objects map[cargo.CompiledRelease]string, downloadThreads int) error {
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	s3Downloader := r.s3Downloader

	for release, path := range matchedS3Objects {
		outputFile := ConvertToLocalBasename(release)

		file, err := os.Create(filepath.Join(releaseDir, outputFile))
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", outputFile, err)
		}

		r.logger.Printf("downloading %s...\n", path)
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
