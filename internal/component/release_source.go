package component

import (
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const (
	ReleaseSourceTypeBOSHIO    = "bosh.io"
	ReleaseSourceTypeS3        = "s3"
	DefaultDownloadThreadCount = 0
)

//counterfeiter:generate -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource
type ReleaseSource interface {
	GetMatchedRelease(Requirement) (Lock, bool, error)
	FindReleaseVersion(Requirement) (Lock, bool, error)
	DownloadRelease(releasesDir string, remoteRelease Lock, downloadThreads int) (Local, error)
	ID() string
	Publishable() bool
}

//counterfeiter:generate -o ./fakes/multi_release_source.go --fake-name MultiReleaseSource . MultiReleaseSource
type MultiReleaseSource interface {
	GetMatchedRelease(Requirement) (Lock, bool, error)
	FindReleaseVersion(Requirement) (Lock, bool, error)
	DownloadRelease(releasesDir string, remoteRelease Lock, downloadThreads int) (Local, error)
	FindByID(string) (ReleaseSource, error)
}

//counterfeiter:generate -o ./fakes/release_uploader.go --fake-name ReleaseUploader . ReleaseUploader
type ReleaseUploader interface {
	GetMatchedRelease(Requirement) (Lock, bool, error)
	UploadRelease(spec Requirement, file io.Reader) (Lock, error)
}

//counterfeiter:generate -o ./fakes/remote_pather.go --fake-name RemotePather . RemotePather
type RemotePather interface {
	RemotePath(Requirement) (string, error)
}

type ReleaseSourceRepo struct {
	ReleaseSources []ReleaseSource
}

func NewReleaseSourceRepo(kilnfile cargo.Kilnfile, logger *log.Logger) ReleaseSourceRepo {
	var releaseSources multiReleaseSource

	for _, releaseConfig := range kilnfile.ReleaseSources {
		releaseSources = append(releaseSources, ReleaseSourceFactory(releaseConfig, logger))
	}

	panicIfDuplicateIDs(releaseSources)

	return ReleaseSourceRepo{ReleaseSources: releaseSources}
}

func (repo ReleaseSourceRepo) MultiReleaseSource(allowOnlyPublishable bool) multiReleaseSource {
	var sources []ReleaseSource
	for _, source := range repo.ReleaseSources {
		if !allowOnlyPublishable || source.Publishable() {
			sources = append(sources, source)
		}
	}

	return sources
}

func (repo ReleaseSourceRepo) FindReleaseUploader(sourceID string) (ReleaseUploader, error) {
	var (
		uploader     ReleaseUploader
		availableIDs []string
	)
	for _, src := range repo.ReleaseSources {
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

func (repo ReleaseSourceRepo) FindRemotePather(sourceID string) (RemotePather, error) {
	var (
		pather       RemotePather
		availableIDs []string
	)

	for _, src := range repo.ReleaseSources {
		u, ok := src.(RemotePather)
		if !ok {
			continue
		}
		availableIDs = append(availableIDs, src.ID())
		if src.ID() == sourceID {
			pather = u
			break
		}
	}

	if len(availableIDs) == 0 {
		return nil, errors.New("no path-generating release sources were found in the Kilnfile")
	}

	if pather == nil {
		return nil, fmt.Errorf(
			"could not find a valid matching release source in the Kilnfile, available path-generating sources are: %q",
			availableIDs,
		)
	}

	return pather, nil
}

func ReleaseSourceFactory(releaseConfig cargo.ReleaseSourceConfig, outLogger *log.Logger) ReleaseSource {
	switch releaseConfig.Type {
	case ReleaseSourceTypeBOSHIO:
		id := releaseConfig.ID
		if id == "" {
			id = ReleaseSourceTypeBOSHIO
		}
		return NewBOSHIOReleaseSource(id, releaseConfig.Publishable, "", outLogger)
	case ReleaseSourceTypeS3:
		if releaseConfig.ID == "" {
			releaseConfig.ID = releaseConfig.Bucket
		}
		return NewS3ReleaseSourceFromConfig(releaseConfig, outLogger)
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
