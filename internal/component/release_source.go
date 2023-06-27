package component

import (
	"fmt"
	"io"
	"log"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

// MultiReleaseSource wraps a set of release sources. It is mostly used to generate fakes
// for testing commands. See ReleaseSourceList for the concrete implementation.
type MultiReleaseSource interface {
	GetMatchedRelease(cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error)
	FindReleaseVersion(spec cargo.BOSHReleaseTarballSpecification, noDownload bool) (cargo.BOSHReleaseTarballLock, error)
	DownloadRelease(releasesDir string, remoteRelease cargo.BOSHReleaseTarballLock) (Local, error)

	FindByID(string) (ReleaseSource, error)

	// SetDownloadThreads allows configuring the concurrency for the s3 release source.
	SetDownloadThreads(n int)
}

//counterfeiter:generate -o ./fakes/multi_release_source.go --fake-name MultiReleaseSource . MultiReleaseSource

// ReleaseUploader represents a place to put releases. Some implementations of ReleaseSource
// should implement this interface. Credentials for this should come from an interpolated
// cargo.ReleaseSourceConfig.
type ReleaseUploader interface {
	GetMatchedRelease(cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error)
	UploadRelease(spec cargo.BOSHReleaseTarballSpecification, file io.Reader) (cargo.BOSHReleaseTarballLock, error)
}

//counterfeiter:generate -o ./fakes/release_uploader.go --fake-name ReleaseUploader . ReleaseUploader

// RemotePather is used to get the remote path for a remote release. For example
// the complete s3 uri.
//
// This interface may be ripe for removal.
type RemotePather interface {
	RemotePath(cargo.BOSHReleaseTarballSpecification) (string, error)
}

//counterfeiter:generate -o ./fakes/remote_pather.go --fake-name RemotePather . RemotePather

// ReleaseSource represents a source where a tile component BOSH releases may come from.
// The releases may be compiled or just built bosh releases.
type ReleaseSource interface {
	// Configuration returns the configuration of the ReleaseSource that came from the kilnfile.
	// It should not be modified.
	Configuration() cargo.ReleaseSourceConfig

	// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
	// fields on Requirement to download a specific release.
	GetMatchedRelease(cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error)

	// FindReleaseVersion may use any of the fields on Requirement to return the best matching
	// release.
	FindReleaseVersion(spec cargo.BOSHReleaseTarballSpecification, noDownload bool) (cargo.BOSHReleaseTarballLock, error)

	// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
	// It should also calculate and set the SHA1 field on the Local result; it does not need
	// to ensure the sums match, the caller must verify this.
	DownloadRelease(releasesDir string, remoteRelease cargo.BOSHReleaseTarballLock) (Local, error)
}

//counterfeiter:generate -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource

const (
	panicMessageWrongReleaseSourceType = "wrong constructor for release source configuration"
	logLineDownload                    = "downloading %s from %s release source %s"
)

// TODO: use the constants from "cargo" everywhere
const (
	ReleaseSourceTypeBOSHIO      = cargo.BOSHReleaseTarballSourceTypeBOSHIO
	ReleaseSourceTypeS3          = cargo.BOSHReleaseTarballSourceTypeS3
	ReleaseSourceTypeGithub      = cargo.BOSHReleaseTarballSourceTypeGithub
	ReleaseSourceTypeArtifactory = cargo.BOSHReleaseTarballSourceTypeArtifactory
)

// ReleaseSourceFactory returns a configured ReleaseSource based on the Type field on the
// cargo.ReleaseSourceConfig structure.
func ReleaseSourceFactory(releaseConfig cargo.ReleaseSourceConfig, outLogger *log.Logger) ReleaseSource {
	releaseConfig.ID = cargo.BOSHReleaseTarballSourceID(releaseConfig)
	switch releaseConfig.Type {
	case ReleaseSourceTypeBOSHIO:
		return NewBOSHIOReleaseSource(releaseConfig, "", outLogger)
	case ReleaseSourceTypeS3:
		return NewS3ReleaseSourceFromConfig(releaseConfig, outLogger)
	case ReleaseSourceTypeGithub:
		return NewGithubReleaseSource(releaseConfig)
	case ReleaseSourceTypeArtifactory:
		return NewArtifactoryReleaseSource(releaseConfig)
	default:
		panic(fmt.Sprintf("unknown release config: %v", releaseConfig))
	}
}
