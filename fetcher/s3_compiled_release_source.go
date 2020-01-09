package fetcher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pivotal-cf/kiln/release"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	ReleaseName     = "release_name"
	ReleaseVersion  = "release_version"
	StemcellOS      = "stemcell_os"
	StemcellVersion = "stemcell_version"
)

type S3CompiledReleaseSource S3ReleaseSource

func (r S3CompiledReleaseSource) ID() string {
	return r.Bucket
}

func (r S3CompiledReleaseSource) GetMatchedReleases(desiredReleaseSet release.ReleaseRequirementSet) ([]release.DeprecatedRemoteRelease, error) {
	matchedS3Objects := make(map[release.ReleaseID][]release.DeprecatedRemoteRelease)

	exp, err := regexp.Compile(r.Regex)
	if err != nil {
		return nil, err
	}
	var count int
	for _, name := range exp.SubexpNames() {
		if name == ReleaseName || name == ReleaseVersion || name == StemcellOS || name == StemcellVersion {
			count++
		}
	}
	if count != 4 {
		return nil, fmt.Errorf("Missing some capture group. Required capture groups: %s, %s, %s, %s", ReleaseName, ReleaseVersion, StemcellOS, StemcellVersion)
	}

	if err := r.S3Client.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: aws.String(r.Bucket),
		},
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			for _, s3Object := range page.Contents {
				if s3Object.Key == nil {
					continue
				}

				compiledRelease, err := createCompiledReleaseFromS3Key(exp, r.Bucket, *s3Object.Key)
				if err != nil {
					continue
				}

				matchedS3Objects[compiledRelease.ReleaseID()] = append(matchedS3Objects[compiledRelease.ReleaseID()], compiledRelease)
			}
			return true
		},
	); err != nil {
		return nil, err
	}

	matchingReleases := make([]release.DeprecatedRemoteRelease, 0)
	for expectedReleaseID, requirement := range desiredReleaseSet {
		if releases, ok := matchedS3Objects[expectedReleaseID]; ok {
			for _, release := range releases {
				if release.Satisfies(requirement) {
					matchingReleases = append(matchingReleases, release)
					break
				}
			}
		}
	}

	return matchingReleases, nil
}

func (r S3CompiledReleaseSource) DownloadReleases(releaseDir string, remoteReleases []release.RemoteRelease, downloadThreads int) (release.LocalReleaseSet, error) {
	releases := make(release.LocalReleaseSet)

	r.Logger.Printf("downloading %d objects from compiled s3...", len(remoteReleases))
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

		r.Logger.Printf("downloading %s...\n", rel.RemotePath)

		_, err = r.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(r.Bucket),
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

func createCompiledReleaseFromS3Key(exp *regexp.Regexp, releaseSourceID, s3Key string) (release.DeprecatedRemoteRelease, error) {
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

	return release.NewCompiledRelease(
		release.ReleaseID{Name: subgroup[ReleaseName], Version: subgroup[ReleaseVersion]},
		subgroup[StemcellOS],
		subgroup[StemcellVersion],
	).WithRemote(releaseSourceID, s3Key), nil
}
