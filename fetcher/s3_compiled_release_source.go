package fetcher

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func (r S3ReleaseSource) GetMatchedReleases(desiredReleaseSet ReleaseSet) (ReleaseSet, error) {
	matchedS3Objects := make(ReleaseSet)

	regex, err := NewReleasesRegexp(r.Regex)
	if err != nil {
		return nil, err
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

				matchedS3Objects[compiledRelease.ID] = CompiledRelease{
					ID:              compiledRelease.ID,
					StemcellVersion: compiledRelease.StemcellVersion,
					StemcellOS:      compiledRelease.StemcellOS,
					Path:            *s3Object.Key,
				}
			}
			return true
		},
	)
	if err != nil {
		return nil, err
	}

	matchingReleases := make(ReleaseSet, 0)
	for expectedRelease := range desiredReleaseSet {
		s3Key, ok := matchedS3Objects[expectedRelease]

		if ok {
			matchingReleases[expectedRelease] = s3Key
		}
	}

	return matchingReleases, nil
}

func (r S3ReleaseSource) DownloadReleases(releaseDir string, matchedS3Objects ReleaseSet,
	downloadThreads int) error {
	r.Logger.Printf("downloading %d objects from s3...", len(matchedS3Objects))
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	for _, release := range matchedS3Objects {
		//Type switch? on release
		compiledRelease := release.(CompiledRelease)
		outputFile := ConvertToLocalBasename(compiledRelease)

		file, err := os.Create(filepath.Join(releaseDir, outputFile))
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", outputFile, err)
		}

		r.Logger.Printf("downloading %s...\n", release.DownloadString())
		_, err = r.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(r.Bucket),
			Key:    aws.String(release.DownloadString()),
		}, setConcurrency)

		if err != nil {
			return fmt.Errorf("failed to download file, %v\n", err)
		}
	}

	return nil
}
