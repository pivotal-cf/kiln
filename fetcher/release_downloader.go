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

func (rd releaseDownloader) DownloadRelease(releaseDir string, requirement release.ReleaseRequirement) (release.ReleaseWithLocation, error) {
	releaseID := release.ReleaseID{Name: requirement.Name, Version: requirement.Version}
	releaseRequirementSet := release.ReleaseRequirementSet{releaseID: requirement}

	for _, releaseSource := range rd.releaseSources {
		remoteReleases, err := releaseSource.GetMatchedReleases(releaseRequirementSet)
		if err != nil {
			return nil, err
		}

		if len(remoteReleases) == 0 {
			continue
		}

		localReleases, err := releaseSource.DownloadReleases(releaseDir, remoteReleases, 0)
		if err != nil {
			return nil, err
		}
		return localReleases[releaseID], nil
	}

	return nil, fmt.Errorf("couldn't find %q %s in any release source", requirement.Name, requirement.Version)
}
