package fetcher

import (
	"fmt"

	"github.com/pivotal-cf/kiln/release"
)

type releaseDownloader struct {
	releaseSources []ReleaseSource
}

func NewReleaseDownloader(releaseSources []ReleaseSource) releaseDownloader {
	return releaseDownloader{releaseSources: releaseSources}
}

func (rd releaseDownloader) DownloadRelease(releaseDir string, requirement release.Requirement) (release.Local, release.Remote, error) {
	for _, releaseSource := range rd.releaseSources {
		remoteRelease, found, err := releaseSource.GetMatchedRelease(requirement)
		if err != nil {
			return release.Local{}, release.Remote{}, err
		}

		if !found {
			continue
		}

		localRelease, err := releaseSource.DownloadRelease(releaseDir, remoteRelease, 0)
		if err != nil {
			return release.Local{}, release.Remote{}, err
		}

		return localRelease, remoteRelease, nil
	}

	return release.Local{}, release.Remote{}, fmt.Errorf("couldn't find %q %s in any release source", requirement.Name, requirement.Version)
}
