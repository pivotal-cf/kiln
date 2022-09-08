package component

import (
	"context"
	"fmt"
	"io"
	"log"
)

const (
	// ReleaseSourceTypeBOSHIO is the value of the Type field on cargo.SharedReleaseSourceConfiguration
	// for fetching https://bosh.io releases.
	ReleaseSourceTypeBOSHIO = "bosh.io"

	// ReleaseSourceTypeS3 is the value for the Type field on cargo.SharedReleaseSourceConfiguration
	// for releases stored on
	ReleaseSourceTypeS3 = "s3"

	// ReleaseSourceTypeGithub is the value for the Type field on cargo.SharedReleaseSourceConfiguration
	// for releases stored on GitHub.
	ReleaseSourceTypeGithub = "github"

	// ReleaseSourceTypeArtifactory is the value for the Type field on cargo.SharedReleaseSourceConfiguration
	// for releases stored on Artifactory.
	ReleaseSourceTypeArtifactory = "artifactory"
)

// ReleaseSource represents a source where a tile component BOSH releases may come from.
// The releases may be compiled or just built bosh releases.
//
//counterfeiter:generate -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource
type ReleaseSource interface {
	ID() string
	Type() string
	ConfigurationErrors() []error
	IsPublishable() bool

	// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
	// fields on Requirement to download a specific release.
	GetMatchedRelease(ctx context.Context, logger *log.Logger, spec Spec) (Lock, error)

	// FindReleaseVersion may use any of the fields on Requirement to return the best matching
	// release.
	FindReleaseVersion(ctx context.Context, logger *log.Logger, spec Spec) (Lock, error)

	// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
	// It should also calculate and set the SHA1 field on the Local result; it does not need
	// to ensure the sums match, the caller must verify this.
	DownloadRelease(ctx context.Context, logger *log.Logger, releasesDir string, remoteRelease Lock) (Local, error)
}

var (
	// this statement ensures the following types implement ReleaseSource
	_ = []ReleaseSource{
		(*S3ReleaseSource)(nil),
		(*ArtifactoryReleaseSource)(nil),
		(*GitHubReleaseSource)(nil),
		(*BOSHIOReleaseSource)(nil),
	}

	_ = []ReleaseUploader{
		(*S3ReleaseSource)(nil),
		(*ArtifactoryReleaseSource)(nil),
	}
)

func NewReleaseSourceLogger(src ReleaseSource, w io.Writer) *log.Logger {
	switch src.Type() {
	case ReleaseSourceTypeArtifactory:
		return log.New(w, fmt.Sprintf("[Artifactory release source %q] ", src.ID()), log.Default().Flags())
	case ReleaseSourceTypeS3:
		return log.New(w, fmt.Sprintf("[S3 release source %q] ", src.ID()), log.Default().Flags())
	case ReleaseSourceTypeBOSHIO:
		return log.New(w, fmt.Sprintf("[bosh.io release source %q] ", src.ID()), log.Default().Flags())
	case ReleaseSourceTypeGithub:
		return log.New(w, fmt.Sprintf("[Github release source %q] ", src.ID()), log.Default().Flags())
	default:
		return log.New(w, "[Unknown release source] ", log.Default().Flags())
	}
}

type EncodedReleaseSource struct {
	ReleaseSource
}

// MarshalYAML will panic if the ReleaseSource concrete type is not registered in the switch statement
func (e *EncodedReleaseSource) MarshalYAML() (interface{}, error) {
	type enc[RS any] struct {
		Type   string `yaml:"type"`
		Source RS     `yaml:",inline"`
	}
	switch src := e.ReleaseSource.(type) {
	case *BOSHIOReleaseSource:
		return enc[BOSHIOReleaseSource]{Source: *src, Type: src.Type()}, nil
	case *GitHubReleaseSource:
		//goland:noinspection GoVetCopyLock
		return enc[GitHubReleaseSource]{Source: *src, Type: src.Type()}, nil
	case *ArtifactoryReleaseSource:
		return enc[ArtifactoryReleaseSource]{Source: *src, Type: src.Type()}, nil
	case *S3ReleaseSource:
		//goland:noinspection GoVetCopyLock
		return enc[S3ReleaseSource]{Source: *src, Type: src.Type()}, nil
	default:
		panic(fmt.Sprintf("marshal as YAML for release source %q not implmenented", e.ReleaseSource.Type()))
	}
}

func (e *EncodedReleaseSource) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var typeField struct {
		Type string `yaml:"type"`
	}

	if err := unmarshal(&typeField); err != nil {
		return err
	}

	switch typeField.Type {
	case ReleaseSourceTypeBOSHIO:
		e.ReleaseSource = new(BOSHIOReleaseSource)
	case ReleaseSourceTypeGithub:
		e.ReleaseSource = new(GitHubReleaseSource)
	case ReleaseSourceTypeArtifactory:
		e.ReleaseSource = new(ArtifactoryReleaseSource)
	case ReleaseSourceTypeS3:
		e.ReleaseSource = new(S3ReleaseSource)
	default:
		return fmt.Errorf("release source type %q not supported", typeField.Type)
	}

	return unmarshal(e.ReleaseSource)
}

// MultiReleaseSource wraps a set of release sources. It is mostly used to generate fakes
// for testing commands. See ReleaseSources for the concrete implementation.
type MultiReleaseSource interface {
	// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
	// fields on Requirement to download a specific release.
	GetMatchedRelease(ctx context.Context, logger *log.Logger, spec Spec) (Lock, error)

	// FindReleaseVersion may use any of the fields on Requirement to return the best matching
	// release.
	FindReleaseVersion(ctx context.Context, logger *log.Logger, spec Spec) (Lock, error)

	// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
	// It should also calculate and set the SHA1 field on the Local result; it does not need
	// to ensure the sums match, the caller must verify this.
	DownloadRelease(ctx context.Context, logger *log.Logger, releasesDir string, remoteRelease Lock) (Local, error)

	FindByID(string) (ReleaseSource, error)
}

//counterfeiter:generate -o ./fakes/multi_release_source.go --fake-name MultiReleaseSource . MultiReleaseSource

// ReleaseUploader represents a place to put releases. Some implementations of ReleaseSource
// should implement this interface. Credentials for this should come from an interpolated
// cargo.ReleaseSource.
type ReleaseUploader interface {
	ReleaseSource

	UploadRelease(ctx context.Context, logger *log.Logger, spec Spec, file io.Reader) (Lock, error)
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
