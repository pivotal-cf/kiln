package fetcher

import (
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/release"
	"io"
)

type MultiReleaseSource []ReleaseSource

//go:generate counterfeiter -o ./fakes/release_uploader.go --fake-name ReleaseUploader . ReleaseUploader
type ReleaseUploader interface {
	UploadRelease(name, version string, file io.Reader) error
	ReleaseSource
}

func (multiSrc MultiReleaseSource) GetMatchedRelease(requirement release.Requirement) (release.Remote, bool, error) {
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

func (multiSrc MultiReleaseSource) DownloadRelease(releaseDir string, remoteRelease release.Remote, downloadThreads int) (release.Local, error) {
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

func (multiSrc MultiReleaseSource) UploadRelease(name, version, sourceID string, file io.Reader) error {
	var (
		uploader ReleaseUploader
		availableIDs []string
	)
	for _, src := range multiSrc {
		u, ok := src.(ReleaseUploader)
		if !ok {
			continue
		}
		availableIDs = append(availableIDs, u.ID())
		if u.ID() == sourceID {
			uploader = u
			break
		}
	}

	if len(availableIDs) == 0 {
		return errors.New("no upload-capable release sources were found in the Kilnfile")
	}

	if uploader == nil {
		return fmt.Errorf(
			"could not find a valid matching release source in the Kilnfile, available upload-compatible sources are: %q",
			availableIDs,
		)
	}

	err := uploader.UploadRelease(name, version, file)
	if err != nil {
		return scopedError(uploader.ID(), err)
	}

	return nil
}

func scopedError(sourceID string, err error) error {
	return fmt.Errorf("error from release source %q: %w", sourceID, err)
}
