package component

import (
	"fmt"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"io"
	"log"
)

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

//counterfeiter:generate -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource
type ReleaseSource interface {
	Configuration() cargo.ReleaseSourceConfig

	GetMatchedRelease(Requirement) (Lock, bool, error)
	FindReleaseVersion(Requirement) (Lock, bool, error)
	DownloadRelease(releasesDir string, remoteRelease Lock, downloadThreads int) (Local, error)
}

const (
	panicMessageWrongReleaseSourceType = "wrong constructor for release source configuration"

	ReleaseSourceTypeBOSHIO = "bosh.io"
	ReleaseSourceTypeS3     = "s3"
)

func ReleaseSourceFactory(releaseConfig cargo.ReleaseSourceConfig, outLogger *log.Logger) ReleaseSource {
	switch releaseConfig.Type {
	case ReleaseSourceTypeBOSHIO:
		if releaseConfig.ID == "" {
			releaseConfig.ID = ReleaseSourceTypeBOSHIO
		}
		return NewBOSHIOReleaseSource(releaseConfig, "", outLogger)
	case ReleaseSourceTypeS3:
		if releaseConfig.ID == "" {
			releaseConfig.ID = releaseConfig.Bucket
		}
		return NewS3ReleaseSourceFromConfig(releaseConfig, outLogger)
	default:
		panic(fmt.Sprintf("unknown release config: %v", releaseConfig))
	}
}
