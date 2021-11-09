package component

import (
	"log"
	"os"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

// Thinking Names should be an array of repo names we're getting the releases for.
type GHRequirement struct {
	RepoNames   []string
	Releases    []string
	TarballPath string
}

type GithubReleaseSource struct {
	cargo.ReleaseSourceConfig
	Token     string
	serverURI string
	logger    *log.Logger
}

func NewGithubReleaseSource(c cargo.ReleaseSourceConfig, customServerURI string, logger *log.Logger) *GithubReleaseSource {
	if c.Type != "" && c.Type != ReleaseSourceTypeGithub {
		panic(panicMessageWrongReleaseSourceType)
	}
	if customServerURI == "" {
		customServerURI = "https://www.github.com"
	}
	if logger == nil {
		logger = log.New(os.Stdout, "[Github release source] ", log.Default().Flags())
	}
	return &GithubReleaseSource{
		ReleaseSourceConfig: c,
		logger:              logger,
		serverURI:           customServerURI,
	}
}

func NewGithubReleaseSourceFromConfig(config cargo.ReleaseSourceConfig) (_ GithubReleaseSource) {
	return
}

// Configuration returns the configuration of the ReleaseSource that came from the kilnfile.
// It should not be modified.
func (src GithubReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return src.ReleaseSourceConfig
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (GithubReleaseSource) GetMatchedRelease(spec Spec) (Lock, bool, error) {
	panic("blah")
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (GithubReleaseSource) FindReleaseVersion(Spec) (Lock, bool, error) {
	panic("blah")
}

// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
// It should also calculate and set the SHA1 field on the Local result; it does not need
// to ensure the sums match, the caller must verify this.
func (GithubReleaseSource) DownloadRelease(releasesDir string, remoteRelease Lock) (Local, error) {
	panic("blah")
}
