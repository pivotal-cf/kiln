package fetcher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pivotal-cf/kiln/internal/cargo"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type BuiltRelease struct {
	ID   ReleaseID
	Path string
}

func (br BuiltRelease) DownloadString() string {
	return br.Path
}

type S3BuiltReleaseSource S3ReleaseSource

func (src S3BuiltReleaseSource) GetMatchedReleases(desiredReleaseSet ReleaseSet, stemcell cargo.Stemcell) (ReleaseSet, error) {
	matchedS3Objects := make(ReleaseSet)

	exp, err := regexp.Compile(src.Regex)
	if err != nil {
		return nil, err
	}
	var count int
	for _, name := range exp.SubexpNames() {
		if name == ReleaseName || name == ReleaseVersion {
			count++
		}
	}
	if count != 2 {
		return nil, fmt.Errorf("Missing some capture group. Required capture groups: %s, %s", ReleaseName, ReleaseVersion)
	}

	if err := src.S3Client.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: aws.String(src.Bucket),
		},
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			for _, s3Object := range page.Contents {
				if s3Object.Key == nil {
					continue
				}
				release, err := createBuiltReleaseFromS3Key(exp, *s3Object.Key)
				if err != nil {
					continue
				}
				matchedS3Objects[release.ID] = release
			}
			return true
		},
	); err != nil {
		return nil, err
	}

	matchingReleases := make(ReleaseSet, 0)
	for expectedReleaseID := range desiredReleaseSet {
		if rel, ok := matchedS3Objects[expectedReleaseID]; ok {
			matchingReleases[expectedReleaseID] = rel
		}
	}

	return matchingReleases, err
}

func (src S3BuiltReleaseSource) DownloadReleases(releaseDir string, matchedS3Objects ReleaseSet, downloadThreads int) error {
	src.Logger.Printf("downloading %d objects from built s3...", len(matchedS3Objects))
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	var errs []error
	for _, release := range matchedS3Objects {
		outputFile, err := ConvertToLocalBasename(release)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		file, err := os.Create(filepath.Join(releaseDir, outputFile))
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", outputFile, err)
		}

		src.Logger.Printf("downloading %s...\n", release.DownloadString())
		_, err = src.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(src.Bucket),
			Key:    aws.String(release.DownloadString()),
		}, setConcurrency)

		if err != nil {
			return fmt.Errorf("failed to download file, %v\n", err)
		}
	}
	if len(errs) > 0 {
		return multipleErrors(errs)
	}
	return nil
}

func createBuiltReleaseFromS3Key(exp *regexp.Regexp, s3Key string) (BuiltRelease, error) {
	if !exp.MatchString(s3Key) {
		return BuiltRelease{}, fmt.Errorf("s3 key does not match regex")
	}

	matches := exp.FindStringSubmatch(s3Key)
	subgroup := make(map[string]string)
	for i, name := range exp.SubexpNames() {
		if i != 0 && name != "" {
			subgroup[name] = matches[i]
		}
	}

	return BuiltRelease{
		ID: ReleaseID{
			Name:    subgroup[ReleaseName],
			Version: subgroup[ReleaseVersion],
		},
		Path: s3Key,
	}, nil
}
