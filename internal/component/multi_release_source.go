package component

import (
	"fmt"

	"github.com/Masterminds/semver"
)

type multiReleaseSource []ReleaseSource

func NewMultiReleaseSource(sources ...ReleaseSource) multiReleaseSource {
	return sources
}

func (multiSrc multiReleaseSource) GetMatchedRelease(requirement Requirement) (Lock, bool, error) {
	for _, src := range multiSrc {
		rel, found, err := src.GetMatchedRelease(requirement)
		if err != nil {
			return Lock{}, false, scopedError(src.ID(), err)
		}
		if found {
			return rel, true, nil
		}
	}
	return Lock{}, false, nil
}

func (multiSrc multiReleaseSource) DownloadRelease(releaseDir string, remoteRelease Lock, downloadThreads int) (Local, error) {
	src, err := multiSrc.FindByID(remoteRelease.RemoteSource)
	if err != nil {
		return Local{}, err
	}

	localRelease, err := src.DownloadRelease(releaseDir, remoteRelease, downloadThreads)
	if err != nil {
		return Local{}, scopedError(src.ID(), err)
	}

	return localRelease, nil
}

func (multiSrc multiReleaseSource) FindReleaseVersion(requirement Requirement) (Lock, bool, error) {
	foundRelease := Lock{}
	releaseWasFound := false
	for _, src := range multiSrc {
		rel, found, err := src.FindReleaseVersion(requirement)
		if err != nil {
			return Lock{}, false, scopedError(src.ID(), err)
		}
		if found {
			releaseWasFound = true
			if (foundRelease == Lock{}) {
				foundRelease = rel
			} else {
				newVersion, _ := semver.NewVersion(rel.Version)
				currentVersion, _ := semver.NewVersion(foundRelease.Version)
				if currentVersion.LessThan(newVersion) {
					foundRelease = rel
				}
			}
		}
	}
	return foundRelease, releaseWasFound, nil
}

func (multiSrc multiReleaseSource) FindByID(id string) (ReleaseSource, error) {
	var correctSrc ReleaseSource
	for _, src := range multiSrc {
		if src.ID() == id {
			correctSrc = src
			break
		}
	}

	if correctSrc == nil {
		ids := make([]string, 0, len(multiSrc))
		for _, src := range multiSrc {
			ids = append(ids, src.ID())
		}
		return nil, fmt.Errorf("couldn't find a release source with ID %q. Available choices: %q", id, ids)
	}

	return correctSrc, nil
}

func scopedError(sourceID string, err error) error {
	return fmt.Errorf("error from release source %q: %w", sourceID, err)
}
