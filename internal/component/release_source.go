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
	GetMatchedRelease(Spec) (Lock, error)
	FindReleaseVersion(Spec) (Lock, error)
	DownloadRelease(releasesDir string, remoteRelease Lock) (Local, error)

	FindByID(string) (ReleaseSource, error)

	// SetDownloadThreads allows configuring the concurrency for the s3 release source.
	SetDownloadThreads(n int)
}

//counterfeiter:generate -o ./fakes/multi_release_source.go --fake-name MultiReleaseSource . MultiReleaseSource

// ReleaseUploader represents a place to put releases. Some implementations of ReleaseSource
// should implement this interface. Credentials for this should come from an interpolated
// cargo.ReleaseSourceConfig.
type ReleaseUploader interface {
	GetMatchedRelease(Spec) (Lock, error)
	UploadRelease(spec Spec, file io.Reader) (Lock, error)
}

//counterfeiter:generate -o ./fakes/release_uploader.go --fake-name ReleaseUploader . ReleaseUploader

// RemotePather is used to get the remote path for a remote release. For example
// the complete s3 uri.
//
// This interface may be ripe for removal.
type RemotePather interface {
	RemotePath(Spec) (string, error)
}

//counterfeiter:generate -o ./fakes/remote_pather.go --fake-name RemotePather . RemotePather

// ReleaseSource represents a source where a tile component BOSH releases may come from.
// The releases may be compiled or just built bosh releases.
type ReleaseSource interface {
	cargo.ReleaseSource

	// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
	// fields on Requirement to download a specific release.
	GetMatchedRelease(Spec) (Lock, error)

	// FindReleaseVersion may use any of the fields on Requirement to return the best matching
	// release.
	FindReleaseVersion(Spec) (Lock, error)

	// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
	// It should also calculate and set the SHA1 field on the Local result; it does not need
	// to ensure the sums match, the caller must verify this.
	DownloadRelease(releasesDir string, remoteRelease Lock) (Local, error)
}

//counterfeiter:generate -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource

const (
	panicMessageWrongReleaseSourceType = "wrong constructor for release source configuration"
	logLineDownload                    = "downloading %s from %s release source %s"

	// ReleaseSourceTypeBOSHIO is the value of the Type field on cargo.ReleaseSourceConfig
	// for fetching https://bosh.io releases.
	ReleaseSourceTypeBOSHIO = cargo.ReleaseSourceTypeBOSHIO

	// ReleaseSourceTypeS3 is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on
	ReleaseSourceTypeS3 = cargo.ReleaseSourceTypeS3

	// ReleaseSourceTypeGithub is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on Github.
	ReleaseSourceTypeGithub = cargo.ReleaseSourceTypeGithub
)

// ReleaseSourceFactory returns a configured ReleaseSource based on the Type field on the
// cargo.ReleaseSourceConfig structure.
func ReleaseSourceFactory(rc cargo.ReleaseSource, outLogger *log.Logger) ReleaseSource {
	switch releaseConfig := rc.(type) {
	case cargo.BOSHIOReleaseSource:
		return NewBOSHIOReleaseSource(releaseConfig, "", outLogger)
	case cargo.S3ReleaseSource:
		return NewS3ReleaseSourceFromConfig(releaseConfig, outLogger)
	case cargo.GitHubReleaseSource:
		return NewGithubReleaseSource(releaseConfig)
	default:
		panic(fmt.Sprintf("unknown release config: %v", releaseConfig))
	}
}
