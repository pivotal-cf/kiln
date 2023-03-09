package component

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

// MultiReleaseSource wraps a set of release sources. It is mostly used to generate fakes
// for testing commands. See ReleaseSourceList for the concrete implementation.
type MultiReleaseSource interface {
	GetMatchedRelease(context.Context, Spec) (Lock, error)
	FindReleaseVersion(ctx context.Context, spec Spec, noDownload bool) (Lock, error)
	DownloadRelease(ctx context.Context, releasesDir string, remoteRelease Lock) (Local, error)

	FindByID(string) (ReleaseSource, error)

	// SetDownloadThreads allows configuring the concurrency for the s3 release source.
	SetDownloadThreads(n int)
}

//counterfeiter:generate -o ./fakes/multi_release_source.go --fake-name MultiReleaseSource . MultiReleaseSource

// ReleaseUploader represents a place to put releases. Some implementations of ReleaseSource
// should implement this interface. Credentials for this should come from an interpolated
// cargo.ReleaseSourceConfig.
type ReleaseUploader interface {
	GetMatchedRelease(context.Context, Spec) (Lock, error)
	UploadRelease(ctx context.Context, spec Spec, file io.Reader) (Lock, error)
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
	// Configuration returns the configuration of the ReleaseSource that came from the kilnfile.
	// It should not be modified.
	Configuration() cargo.ReleaseSourceConfig

	// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
	// fields on Requirement to download a specific release.
	GetMatchedRelease(context.Context, Spec) (Lock, error)

	// FindReleaseVersion may use any of the fields on Requirement to return the best matching
	// release.
	FindReleaseVersion(ctx context.Context, pec Spec, noDownload bool) (Lock, error)

	// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
	// It should also calculate and set the SHA1 field on the Local result; it does not need
	// to ensure the sums match, the caller must verify this.
	DownloadRelease(ctx context.Context, releasesDir string, remoteRelease Lock) (Local, error)
}

//counterfeiter:generate -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource

const (
	panicMessageWrongReleaseSourceType = "wrong constructor for release source configuration"
	logLineDownload                    = "downloading %s from %s release source %s"
)

// TODO: use the constants from "cargo" everywhere
const (
	ReleaseSourceTypeBOSHIO      = cargo.ReleaseSourceTypeBOSHIO
	ReleaseSourceTypeS3          = cargo.ReleaseSourceTypeS3
	ReleaseSourceTypeGithub      = cargo.ReleaseSourceTypeGithub
	ReleaseSourceTypeArtifactory = cargo.ReleaseSourceTypeArtifactory
)

// ReleaseSourceFactory returns a configured ReleaseSource based on the Type field on the
// cargo.ReleaseSourceConfig structure.
func ReleaseSourceFactory(releaseConfig cargo.ReleaseSourceConfig, outLogger *log.Logger) ReleaseSource {
	releaseConfig.ID = cargo.ReleaseSourceID(releaseConfig)
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
