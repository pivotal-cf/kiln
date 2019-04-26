package fetcher

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

const baseRegex = `^%s/.+/(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\.]+(-\w+)??)-(?P<stemcell_os>([a-z_]*-?){1,2})-(?P<stemcell_version>\d+\.\d+)(\.0)?\.tgz$`

type ReleaseMatcher struct {
	s3Provider s3Provider
}

func NewReleaseMatcher(s3Provider s3Provider) ReleaseMatcher {
	return ReleaseMatcher{
		s3Provider: s3Provider,
	}
}

//go:generate counterfeiter -o ./fakes/s3client.go --fake-name S3Client github.com/pivotal-cf/kiln/vendor/github.com/aws/aws-sdk-go/service/s3/s3iface.S3API
func (r ReleaseMatcher) GetMatchedReleases(compiledReleases cargo.CompiledReleases, assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, error) {
	matchedS3Objects := make(map[cargo.CompiledRelease]string)

	regex, err := NewCompiledReleasesRegexp(fmt.Sprintf(baseRegex, compiledReleases.PASVersion))
	if err != nil {
		return nil, err
	}

	s3Client := r.s3Provider.GetS3Client(compiledReleases.Region, compiledReleases.AccessKeyId, compiledReleases.SecretAccessKey)

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
		return nil, err
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
	if len(missingReleases) > 0 {
		formattedMissingReleases := make([]string, 0)

		for _, missingRelease := range missingReleases {
			formattedMissingReleases = append(formattedMissingReleases, fmt.Sprintf(
				"%+v", missingRelease,
			))

		}
		return nil, fmt.Errorf("Expected releases were not matched by the regex:\n%s", strings.Join(formattedMissingReleases, "\n"))
	}

	return matchingReleases, nil
}
