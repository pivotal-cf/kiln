package fetcher

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

//go:generate counterfeiter -o ./fakes/s3_downloader.go --fake-name S3Downloader . S3Downloader
type S3Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

//go:generate counterfeiter -o ./fakes/s3_object_lister.go --fake-name S3ObjectLister . S3ObjectLister
type S3ObjectLister interface {
	ListObjectsPages(*s3.ListObjectsInput, func(*s3.ListObjectsOutput, bool) bool) error
}

type S3ReleaseSource struct {
	Logger       *log.Logger
	S3Client     S3ObjectLister
	S3Downloader S3Downloader
	Bucket       string
	Regex        string
}

func (r *S3ReleaseSource) Configure(assets cargo.Assets) {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(assets.CompiledReleases.Region),
		Credentials: credentials.NewStaticCredentials(
			assets.CompiledReleases.AccessKeyId,
			assets.CompiledReleases.SecretAccessKey,
			"",
		),
	}))
	client := s3.New(sess)

	r.S3Client = client
	r.S3Downloader = s3manager.NewDownloaderWithClient(client)

	r.Bucket = assets.CompiledReleases.Bucket
	r.Regex = assets.CompiledReleases.Regex
}

func (r *S3ReleaseSource) GetMatchedReleases(assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, []cargo.CompiledRelease, error) {
	matchedS3Objects := make(map[cargo.CompiledRelease]string)

	regex, err := NewCompiledReleasesRegexp(r.Regex)
	if err != nil {
		return nil, nil, err
	}

	err = r.S3Client.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: aws.String(r.Bucket),
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

func (r *S3ReleaseSource) DownloadReleases(releaseDir string, matchedS3Objects map[cargo.CompiledRelease]string,
	downloadThreads int) error {
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	for release, path := range matchedS3Objects {
		outputFile := ConvertToLocalBasename(release)

		file, err := os.Create(filepath.Join(releaseDir, outputFile))
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", outputFile, err)
		}

		r.Logger.Printf("downloading %s...\n", path)
		_, err = r.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(r.Bucket),
			Key:    aws.String(path),
		}, setConcurrency)

		if err != nil {
			return fmt.Errorf("failed to download file, %v\n", err)
		}
	}

	return nil
}
