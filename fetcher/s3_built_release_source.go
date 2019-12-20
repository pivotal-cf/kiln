package fetcher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3BuiltReleaseSource S3ReleaseSource

func (src S3BuiltReleaseSource) GetMatchedReleases(desiredReleaseSet ReleaseRequirementSet) ([]RemoteRelease, error) {
	matchedS3Objects := make(map[ReleaseID]BuiltRelease)

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

	matchingReleases := make([]RemoteRelease, 0)
	for expectedReleaseID := range desiredReleaseSet {
		if rel, ok := matchedS3Objects[expectedReleaseID]; ok {
			matchingReleases = append(matchingReleases, rel)
		}
	}

	return matchingReleases, err
}

func (src S3BuiltReleaseSource) DownloadReleases(releaseDir string, remoteReleases []RemoteRelease, downloadThreads int) (LocalReleaseSet, error) {
	localReleases := make(LocalReleaseSet)

	src.Logger.Printf("downloading %d objects from built s3...", len(remoteReleases))
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	var errs []error
	for _, release := range remoteReleases {
		outputFile := filepath.Join(releaseDir, release.StandardizedFilename())
		file, err := os.Create(outputFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %q: %w", outputFile, err)
		}

		src.Logger.Printf("downloading %s...\n", release.RemotePath())
		_, err = src.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(src.Bucket),
			Key:    aws.String(release.RemotePath()),
		}, setConcurrency)

		if err != nil {
			return nil, fmt.Errorf("failed to download file: %w\n", err)
		}
		rID := release.ReleaseID()
		localReleases[rID] = release.AsLocal(outputFile)
	}
	if len(errs) > 0 {
		return nil, multipleErrors(errs)
	}
	return localReleases, nil
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

	return NewBuiltRelease(
		ReleaseID{Name: subgroup[ReleaseName], Version: subgroup[ReleaseVersion]},
		"",
		s3Key,
	), nil
}
