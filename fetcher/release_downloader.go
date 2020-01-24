package fetcher

import (
	"fmt"

	"github.com/pivotal-cf/kiln/release"
)

type releaseDownloader MultiReleaseSource

func NewReleaseDownloader(releaseSource MultiReleaseSource) releaseDownloader {
	return releaseDownloader(releaseSource)
}

func (rd releaseDownloader) DownloadRelease(releaseDir string, requirement release.Requirement) (release.Local, release.Remote, error) {
		remoteRelease, found, err := MultiReleaseSource(rd).GetMatchedRelease(requirement)
		if err != nil {
			return release.Local{}, release.Remote{}, err
		}

		if !found {
			return release.Local{}, release.Remote{}, fmt.Errorf("couldn't find %q %s in any release source", requirement.Name, requirement.Version)
		}

		localRelease, err := MultiReleaseSource(rd).DownloadRelease(releaseDir, remoteRelease, 0)
		if err != nil {
			return release.Local{}, release.Remote{}, err
		}

		return localRelease, remoteRelease, nil
}
