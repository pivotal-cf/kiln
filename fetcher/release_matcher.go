package fetcher

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

type ReleaseMatcher struct {
	s3Provider s3Provider
}

func NewReleaseMatcher(s3Provider s3Provider) ReleaseMatcher {
	return ReleaseMatcher{
		s3Provider: s3Provider,
	}
}

//go:generate counterfeiter -o ./fakes/s3client.go --fake-name S3Client github.com/pivotal-cf/kiln/vendor/github.com/aws/aws-sdk-go/service/s3/s3iface.S3API
func (r ReleaseMatcher) GetMatchedReleases(compiledReleases cargo.CompiledReleases, assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, []cargo.CompiledRelease, error) {
	matchedS3Objects := make(map[cargo.CompiledRelease]string)

	regex, err := NewCompiledReleasesRegexp(compiledReleases.Regex)
	if err != nil {
		return nil, nil, err
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
