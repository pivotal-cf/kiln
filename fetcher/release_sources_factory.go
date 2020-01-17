package fetcher

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/kiln/release"

	"github.com/pivotal-cf/kiln/internal/cargo"
)

const (
	ReleaseSourceTypeBOSHIO = "bosh.io"
	ReleaseSourceTypeS3     = "s3"
)

//go:generate counterfeiter -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource
type ReleaseSource interface {
	GetMatchedRelease(release.Requirement) (release.Remote, bool, error)
	DownloadRelease(releasesDir string, remoteRelease release.Remote, downloadThreads int) (release.Local, error)
	ID() string
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

		panicIfDuplicateIDs(releaseSources)

		return releaseSources
	}
}

func releaseSourceFor(releaseConfig cargo.ReleaseSourceConfig, outLogger *log.Logger) ReleaseSource {
	switch releaseConfig.Type {
	case ReleaseSourceTypeBOSHIO:
		return NewBOSHIOReleaseSource(outLogger, "")
	case ReleaseSourceTypeS3:
		s3ReleaseSource := S3ReleaseSource{Logger: outLogger}
		s3ReleaseSource.Configure(releaseConfig)
		return s3ReleaseSource
	default:
		panic(fmt.Sprintf("unknown release config: %v", releaseConfig))
	}
}

func panicIfDuplicateIDs(releaseSources []ReleaseSource) {
	indexOfID := make(map[string]int)
	for index, rs := range releaseSources {
		id := rs.ID()
		previousIndex, seen := indexOfID[id]
		if seen {
			panic(fmt.Sprintf(`release_sources must have unique IDs; items at index %d and %d both have ID %q`, previousIndex, index, id))
		}
		indexOfID[id] = index
	}
}
