package fetcher

import (
	"errors"
	"fmt"
	"io"
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

//go:generate counterfeiter -o ./fakes/release_uploader.go --fake-name ReleaseUploader . ReleaseUploader
type ReleaseUploader interface {
	GetMatchedRelease(release.Requirement) (release.Remote, bool, error)
	UploadRelease(name, version string, file io.Reader) error
}

type releaseSourceFunction func(cargo.Kilnfile, bool) MultiReleaseSource

func (rsf releaseSourceFunction) ReleaseSource(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) MultiReleaseSource {
	return rsf(kilnfile, allowOnlyPublishable)
}

func (rsf releaseSourceFunction) ReleaseUploader(sourceID string, kilnfile cargo.Kilnfile) (ReleaseUploader, error) {
	var (
		uploader     ReleaseUploader
		availableIDs []string
	)
	sources := rsf(kilnfile, false)

	for _, src := range sources {
		u, ok := src.(ReleaseUploader)
		if !ok {
			continue
		}
		availableIDs = append(availableIDs, src.ID())
		if src.ID() == sourceID {
			uploader = u
			break
		}
	}

	if len(availableIDs) == 0 {
		return nil, errors.New("no upload-capable release sources were found in the Kilnfile")
	}

	if uploader == nil {
		return nil, fmt.Errorf(
			"could not find a valid matching release source in the Kilnfile, available upload-compatible sources are: %q",
			availableIDs,
		)
	}

	return uploader, nil
}

func NewReleaseSourceFactory(outLogger *log.Logger) releaseSourceFunction {
	return func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) MultiReleaseSource {
		var releaseSources MultiReleaseSource

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
