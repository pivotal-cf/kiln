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

type compiledRelease struct {
	release.ReleaseID
	stemcellOS string
	stemcellVersion string
	remotePath string
}

func (cr compiledRelease) satisfies(rr release.ReleaseRequirement) bool {
	return cr.Name == rr.Name &&
		cr.Version == rr.Version &&
		cr.stemcellOS == rr.StemcellOS &&
		cr.stemcellVersion == cr.stemcellVersion
}

func (cr compiledRelease) asRemoteRelease() release.RemoteRelease {
	return release.RemoteRelease{
		ReleaseID: cr.ReleaseID,
		RemotePath: cr.remotePath,
	}
}

func (r S3CompiledReleaseSource) GetMatchedReleases(desiredReleaseSet release.ReleaseRequirementSet) ([]release.RemoteRelease, error) {
	matchedS3Objects := make(map[release.ReleaseID][]compiledRelease)

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

				matchedS3Objects[compiledRelease.ReleaseID] = append(matchedS3Objects[compiledRelease.ReleaseID], compiledRelease)
			}
			return true
		},
	); err != nil {
		return nil, err
	}

	matchingReleases := make([]release.RemoteRelease, 0)
	for expectedReleaseID, requirement := range desiredReleaseSet {
		if releases, ok := matchedS3Objects[expectedReleaseID]; ok {
			for _, release := range releases {
				if release.satisfies(requirement) {
					matchingReleases = append(matchingReleases, release.asRemoteRelease())
					break
				}
			}
		}
	}

	return matchingReleases, nil
}

func (r S3CompiledReleaseSource) DownloadReleases(releaseDir string, remoteReleases []release.RemoteRelease, downloadThreads int) ([]release.LocalRelease, error) {
	var releases []release.LocalRelease

	r.Logger.Printf("downloading %d objects from compiled s3...", len(remoteReleases))
	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		} else {
			dl.Concurrency = s3manager.DefaultDownloadConcurrency
		}
	}

	for _, rel := range remoteReleases {
		outputFile := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", rel.Name, rel.Version))

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

		releases = append(releases, release.LocalRelease{ReleaseID: rel.ReleaseID, LocalPath: outputFile})
	}

	return releases, nil
}

func createCompiledReleaseFromS3Key(exp *regexp.Regexp, releaseSourceID, s3Key string) (compiledRelease, error) {
	if !exp.MatchString(s3Key) {
		return compiledRelease{}, fmt.Errorf("s3 key does not match regex")
	}

	matches := exp.FindStringSubmatch(s3Key)
	subgroup := make(map[string]string)
	for i, name := range exp.SubexpNames() {
		if i != 0 && name != "" {
			subgroup[name] = matches[i]
		}
	}

	return compiledRelease{
		ReleaseID:       release.ReleaseID{Name: subgroup[ReleaseName], Version: subgroup[ReleaseVersion]},
		stemcellOS:      subgroup[StemcellOS],
		stemcellVersion: subgroup[StemcellVersion],
		remotePath:      s3Key,
	}, nil
}
