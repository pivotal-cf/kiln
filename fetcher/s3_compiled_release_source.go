package fetcher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pivotal-cf/kiln/internal/cargo"

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

type CompiledRelease struct {
	ID              ReleaseID
	StemcellOS      string
	StemcellVersion string
	Path            string
}

func (cr CompiledRelease) DownloadString() string {
	return cr.Path
}

type S3CompiledReleaseSource S3ReleaseSource

func (r S3CompiledReleaseSource) GetMatchedReleases(desiredReleaseSet ReleaseSet, stemcell cargo.Stemcell) (ReleaseSet, error) {
	matchedS3Objects := make(map[ReleaseID][]CompiledRelease)

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

				compiledRelease, err := createCompiledReleaseFromS3Key(exp, *s3Object.Key)
				if err != nil {
					continue
				}

				matchedS3Objects[compiledRelease.ID] = append(matchedS3Objects[compiledRelease.ID], compiledRelease)
			}
			return true
		},
	); err != nil {
		return nil, err
	}

	matchingReleases := make(ReleaseSet, 0)
	for expectedReleaseID := range desiredReleaseSet {
		if releases, ok := matchedS3Objects[expectedReleaseID]; ok {
			for _, release := range releases {
				if release.StemcellVersion == stemcell.Version && release.StemcellOS == stemcell.OS {
					matchingReleases[expectedReleaseID] = release
					break
				}
			}
		}
	}

	return matchingReleases, nil
}

func (r S3CompiledReleaseSource) DownloadReleases(releaseDir string, matchedS3Objects ReleaseSet, downloadThreads int) error {
	r.Logger.Printf("downloading %d objects from compiled s3...", len(matchedS3Objects))
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

		r.Logger.Printf("downloading %s...\n", release.DownloadString())
		_, err = r.S3Downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(r.Bucket),
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

func createCompiledReleaseFromS3Key(exp *regexp.Regexp, s3Key string) (CompiledRelease, error) {
	if !exp.MatchString(s3Key) {
		return CompiledRelease{}, fmt.Errorf("s3 key does not match regex")
	}

	matches := exp.FindStringSubmatch(s3Key)
	subgroup := make(map[string]string)
	for i, name := range exp.SubexpNames() {
		if i != 0 && name != "" {
			subgroup[name] = matches[i]
		}
	}

	return CompiledRelease{
		ID: ReleaseID{
			Name:    subgroup[ReleaseName],
			Version: subgroup[ReleaseVersion],
		},
		StemcellOS:      subgroup[StemcellOS],
		StemcellVersion: subgroup[StemcellVersion],
		Path:            s3Key,
	}, nil
}
