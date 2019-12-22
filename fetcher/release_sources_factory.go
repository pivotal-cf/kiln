package fetcher

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/kiln/release"

	"github.com/pivotal-cf/kiln/internal/cargo"
)

//go:generate counterfeiter -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource
type ReleaseSource interface {
	GetMatchedReleases(release.ReleaseRequirementSet) ([]release.RemoteRelease, error)
	DownloadReleases(releasesDir string, matchedS3Objects []release.RemoteRelease, downloadThreads int) (release.ReleaseWithLocationSet, error)
}

type releaseSourceFunction func(cargo.Kilnfile, bool) []ReleaseSource

func (rsf releaseSourceFunction) ReleaseSources(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) []ReleaseSource {
	return rsf(kilnfile, allowOnlyPublishable)
}

func NewReleaseSourcesFactory(outLogger *log.Logger) releaseSourceFunction {
	return func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) []ReleaseSource {
		var releaseSources []ReleaseSource

		for _, releaseConfig := range kilnfile.ReleaseSources {
			if allowOnlyPublishable && !releaseConfig.Publishable {
				continue
			}
			releaseSources = append(releaseSources, releaseSourceFor(releaseConfig, outLogger))
		}

		return releaseSources
	}
}

func releaseSourceFor(releaseConfig cargo.ReleaseSourceConfig, outLogger *log.Logger) ReleaseSource {
	switch releaseConfig.Type {
	case "bosh.io":
		return NewBOSHIOReleaseSource(outLogger, "")
	case "s3":
		s3ReleaseSource := S3ReleaseSource{Logger: outLogger, ID: releaseConfig.ID}
		s3ReleaseSource.Configure(releaseConfig)
		if releaseConfig.Compiled {
			return S3CompiledReleaseSource(s3ReleaseSource)
		}
		return S3BuiltReleaseSource(s3ReleaseSource)
	default:
		panic(fmt.Sprintf("unknown release config: %v", releaseConfig))
	}
}
