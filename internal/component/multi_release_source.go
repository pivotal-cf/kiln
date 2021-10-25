package component

import (
	"fmt"

	"github.com/Masterminds/semver"

	"github.com/pivotal-cf/kiln/pkg/release"
)

type multiReleaseSource []ReleaseSource

func NewMultiReleaseSource(sources ...ReleaseSource) multiReleaseSource {
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
	src, err := multiSrc.FindByID(remoteRelease.SourceID)
	if err != nil {
		return release.Local{}, err
	}

	localRelease, err := src.DownloadRelease(releaseDir, remoteRelease, downloadThreads)
	if err != nil {
		return release.Local{}, scopedError(src.ID(), err)
	}

	return localRelease, nil
}

func (multiSrc multiReleaseSource) FindReleaseVersion(requirement release.Requirement) (release.Remote, bool, error) {
	foundRelease := release.Remote{}
	releaseWasFound := false
	for _, src := range multiSrc {
		rel, found, err := src.FindReleaseVersion(requirement)
		if err != nil {
			return release.Remote{}, false, scopedError(src.ID(), err)
		}
		if found {
			releaseWasFound = true
			if (foundRelease == release.Remote{}) {
				foundRelease = rel
			} else {
				newVersion, _ := semver.NewVersion(rel.ID.Version)
				currentVersion, _ := semver.NewVersion(foundRelease.ID.Version)
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
