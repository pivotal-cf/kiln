package fetcher

import (
	"log"

	"github.com/pivotal-cf/kiln/commands"

	"github.com/pivotal-cf/kiln/internal/cargo"
)

type BOSHIOReleaseSource struct {
	logger *log.Logger
}

func NewBOSHIOReleaseSource(logger *log.Logger) commands.ReleaseSource {
	return BOSHIOReleaseSource{logger}
}

func (r BOSHIOReleaseSource) GetMatchedReleases(compiledReleases cargo.CompiledReleases, assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, []cargo.CompiledRelease, error) {
	matchedBOSHIOReleases := make(map[cargo.CompiledRelease]string)

	compiledRelease := cargo.CompiledRelease{"bpm", "1.2.3-lts", "ubuntu-xenial", "190.0.0"}
	BOSHIO_URL := "matchedBOSHIOReleases2.5/bpm/bpm-1.2.3-lts-ubuntu-xenial-190.0.0.tgz"
	matchedBOSHIOReleases[compiledRelease] = BOSHIO_URL

	return matchedBOSHIOReleases, nil, nil
}

func (r BOSHIOReleaseSource) DownloadReleases(releaseDir string, compiledReleases cargo.CompiledReleases, matchedBOSHObjects map[cargo.CompiledRelease]string, downloadThreads int) error {
	return nil
}
