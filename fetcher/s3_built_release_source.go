package fetcher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pivotal-cf/kiln/release"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3BuiltReleaseSource S3ReleaseSource

func (src S3BuiltReleaseSource) ID() string {
	return src.Bucket
}

func (src S3BuiltReleaseSource) GetMatchedReleases(desiredReleaseSet release.ReleaseRequirementSet) ([]release.DeprecatedRemoteRelease, error) {
	matchedS3Objects := make(map[release.ReleaseID]release.DeprecatedRemoteRelease)

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
				release, err := createBuiltReleaseFromS3Key(exp, src.Bucket, *s3Object.Key)
				if err != nil {
					continue
				}
				matchedS3Objects[release.ReleaseID()] = release
			}
			return true
		},
	); err != nil {
		return nil, err
	}

	matchingReleases := make([]release.DeprecatedRemoteRelease, 0)
	for expectedReleaseID := range desiredReleaseSet {
		if rel, ok := matchedS3Objects[expectedReleaseID]; ok {
			matchingReleases = append(matchingReleases, rel)
		}
	}

	return matchingReleases, err
}

func (src S3BuiltReleaseSource) DownloadReleases(releaseDir string, remoteReleases []release.RemoteRelease, downloadThreads int) (release.LocalReleaseSet, error) {
	releases := make(release.LocalReleaseSet)

	src.Logger.Printf("downloading %d objects from built s3...", len(remoteReleases))
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	for _, rel := range remoteReleases {
		outputFile := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", rel.ReleaseID.Name, rel.ReleaseID.Version))

		file, err := os.Create(outputFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %q: %w", outputFile, err)
		}

		src.Logger.Printf("downloading %s...\n", rel.RemotePath)

		_, err = src.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(src.Bucket),
			Key:    aws.String(rel.RemotePath),
		}, setConcurrency)
		if err != nil {
			return nil, fmt.Errorf("failed to download file: %w\n", err)
		}

		releases[rel.ReleaseID] = release.LocalRelease{
			ReleaseID: rel.ReleaseID,
			LocalPath: outputFile,
		}
	}

	return releases, nil
}

func createBuiltReleaseFromS3Key(exp *regexp.Regexp, releaseSourceID, s3Key string) (release.DeprecatedRemoteRelease, error) {
	if !exp.MatchString(s3Key) {
		return nil, fmt.Errorf("s3 key does not match regex")
	}

	matches := exp.FindStringSubmatch(s3Key)
	subgroup := make(map[string]string)
	for i, name := range exp.SubexpNames() {
		if i != 0 && name != "" {
			subgroup[name] = matches[i]
		}
	}

	return release.NewBuiltRelease(
		release.ReleaseID{Name: subgroup[ReleaseName], Version: subgroup[ReleaseVersion]},
	).WithRemote(releaseSourceID, s3Key), nil
}
