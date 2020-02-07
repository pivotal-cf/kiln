package fetcher

import (
	"fmt"
	"github.com/pivotal-cf/kiln/release"
)

type multiReleaseSource []ReleaseSource

func NewMultiReleaseSource(sources...ReleaseSource) multiReleaseSource {
	return sources
}

func (multiSrc multiReleaseSource) GetMatchedRelease(requirement release.Requirement) (release.Remote, bool, error) {
	for _, src := range multiSrc {
		rel, found, err := src.GetMatchedRelease(requirement)
		if err != nil {
			return release.Remote{}, false, scopedError(src.ID(), err)
		}
		if found {
			return rel, true, nil
		}
	}
	return release.Remote{}, false, nil
}

func (multiSrc multiReleaseSource) DownloadRelease(releaseDir string, remoteRelease release.Remote, downloadThreads int) (release.Local, error) {
	var correctSrc ReleaseSource
	for _, src := range multiSrc {
		if src.ID() == remoteRelease.SourceID {
			correctSrc = src
			break
		}
	}

	if correctSrc == nil {
		ids := make([]string, 0, len(multiSrc))
		for _, src := range multiSrc {
			ids = append(ids, src.ID())
		}
		return release.Local{}, fmt.Errorf("couldn't find a release source with ID %q. Available choices: %q", remoteRelease.SourceID, ids)
	}

	localRelease, err := correctSrc.DownloadRelease(releaseDir, remoteRelease, downloadThreads)
	if err != nil {
		return release.Local{}, scopedError(correctSrc.ID(), err)
	}

	return localRelease, nil
}

func scopedError(sourceID string, err error) error {
	return fmt.Errorf("error from release source %q: %w", sourceID, err)
}
