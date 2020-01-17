package fetcher

import (
	"fmt"
	"io"
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

func (src S3BuiltReleaseSource) GetMatchedReleases(desiredReleaseSet release.ReleaseRequirementSet) ([]release.RemoteRelease, error) {
	matchedS3Objects := make(map[release.ReleaseID]release.RemoteRelease)

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
				matchedS3Objects[release.ReleaseID] = release
			}
			return true
		},
	); err != nil {
		return nil, err
	}

	matchingReleases := make([]release.RemoteRelease, 0)
	for expectedReleaseID := range desiredReleaseSet {
		if rel, ok := matchedS3Objects[expectedReleaseID]; ok {
			matchingReleases = append(matchingReleases, rel)
		}
	}

	return matchingReleases, err
}

func (src S3BuiltReleaseSource) DownloadReleases(releaseDir string, remoteReleases []release.RemoteRelease, downloadThreads int) ([]release.LocalRelease, error) {
	var releases []release.LocalRelease

	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	for _, rel := range remoteReleases {
		src.Logger.Printf("downloading %s %s from %s", rel.Name, rel.Version, src.Bucket)

		outputFile := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", rel.Name, rel.Version))

		file, err := os.Create(outputFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %q: %w", outputFile, err)
		}

		_, err = src.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(src.Bucket),
			Key:    aws.String(rel.RemotePath),
		}, setConcurrency)
		if err != nil {
			return nil, fmt.Errorf("failed to download file: %w\n", err)
		}

		releases = append(releases, release.LocalRelease{ReleaseID: rel.ReleaseID, LocalPath: outputFile})
	}

	return releases, nil
}

func createBuiltReleaseFromS3Key(exp *regexp.Regexp, releaseSourceID, s3Key string) (release.RemoteRelease, error) {
	if !exp.MatchString(s3Key) {
		return release.RemoteRelease{}, fmt.Errorf("s3 key does not match regex")
	}

	matches := exp.FindStringSubmatch(s3Key)
	subgroup := make(map[string]string)
	for i, name := range exp.SubexpNames() {
		if i != 0 && name != "" {
			subgroup[name] = matches[i]
		}
	}

	return release.RemoteRelease{
		ReleaseID:  release.ReleaseID{Name: subgroup[ReleaseName], Version: subgroup[ReleaseVersion]},
		RemotePath: s3Key,
	}, nil
}

func (src S3BuiltReleaseSource) UploadRelease(name, version string, file io.Reader) error {
	remotePath := fmt.Sprintf("%s/%s-%s.tgz", name, name, version)

	re, err := regexp.Compile(src.Regex)
	if err != nil {
		return fmt.Errorf("couldn't compile the regular expression for release source %q: %w", src.ID(), err)
	}

	if !re.MatchString(remotePath) {
		return fmt.Errorf("remote path %q does not match regular expression for release source %q", remotePath, src.ID())
	}

	src.Logger.Printf("Uploading release to %s at %q...\n", src.ID(), remotePath)

	_, err = src.S3Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(src.Bucket),
		Key:    aws.String(remotePath),
		Body:   file,
	})
	if err != nil {
		return err
	}

	return nil
}
