package component

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type ReleaseSourceList []ReleaseSource

func NewReleaseSourceRepo(kilnfile cargo.Kilnfile) ReleaseSourceList {
	var list ReleaseSourceList

	for _, releaseConfig := range kilnfile.ReleaseSources {
		list = append(list, ReleaseSourceFactory(releaseConfig))
	}

	panicIfDuplicateIDs(list)

	return list
}

func (list ReleaseSourceList) Filter(allowOnlyPublishable bool) ReleaseSourceList {
	var sources ReleaseSourceList
	for _, source := range list {
		if allowOnlyPublishable && !source.Configuration().Publishable {
			continue
		}
		sources = append(sources, source)
	}
	return sources
}

func (list ReleaseSourceList) FindReleaseUploader(sourceID string) (ReleaseUploader, error) {
	var (
		uploader     ReleaseUploader
		availableIDs []string
	)
	for _, src := range list {
		u, ok := src.(ReleaseUploader)
		if !ok {
			continue
		}
		availableIDs = append(availableIDs, src.Configuration().ID)
		if src.Configuration().ID == sourceID {
			uploader = u
			break
		}
	}

	if len(availableIDs) == 0 {
		return nil, errors.New("no upload-capable release sources were found in the Kilnfile")
	}

	if uploader == nil {
		return nil, fmt.Errorf(
			"could not find a valid matching release source in the Kilnfile, available upload-compatible sources are: %q",
			availableIDs,
		)
	}

	return uploader, nil
}

func (list ReleaseSourceList) FindRemotePather(sourceID string) (RemotePather, error) {
	var (
		pather       RemotePather
		availableIDs []string
	)

	for _, src := range list {
		u, ok := src.(RemotePather)
		if !ok {
			continue
		}
		id := src.Configuration().ID
		availableIDs = append(availableIDs, id)
		if id == sourceID {
			pather = u
			break
		}
	}

	if len(availableIDs) == 0 {
		return nil, errors.New("no path-generating release sources were found in the Kilnfile")
	}

	if pather == nil {
		return nil, fmt.Errorf(
			"could not find a valid matching release source in the Kilnfile, available path-generating sources are: %q",
			availableIDs,
		)
	}

	return pather, nil
}

func panicIfDuplicateIDs(releaseSources []ReleaseSource) {
	indexOfID := make(map[string]int)
	for index, rs := range releaseSources {
		id := rs.Configuration().ID
		previousIndex, seen := indexOfID[id]
		if seen {
			panic(fmt.Sprintf(`release_sources must have unique IDs; items at index %d and %d both have ID %q`, previousIndex, index, id))
		}
		indexOfID[id] = index
	}
}

func NewMultiReleaseSource(sources ...ReleaseSource) ReleaseSourceList {
	return sources
}

func (list ReleaseSourceList) GetMatchedRelease(requirement cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
	for _, src := range list {
		rel, err := src.GetMatchedRelease(requirement)
		if err != nil {
			if IsErrNotFound(err) {
				continue
			}
			return cargo.BOSHReleaseTarballLock{}, scopedError(src.Configuration().ID, err)
		}
		return rel, nil
	}
	return cargo.BOSHReleaseTarballLock{}, ErrNotFound
}

func (list ReleaseSourceList) SetDownloadThreads(n int) {
	for i, rs := range list {
		s3rs, ok := rs.(S3ReleaseSource)
		if ok {
			s3rs.DownloadThreads = n
			list[i] = s3rs
		}
	}
}

func (list ReleaseSourceList) DownloadRelease(releaseDir string, remoteRelease cargo.BOSHReleaseTarballLock) (Local, error) {
	src, err := list.FindByID(remoteRelease.RemoteSource)
	if err != nil {
		return Local{}, err
	}

	localRelease, err := src.DownloadRelease(releaseDir, remoteRelease)
	if err != nil {
		return Local{}, scopedError(src.Configuration().ID, err)
	}

	return localRelease, nil
}

func (list ReleaseSourceList) FindReleaseVersion(requirement cargo.BOSHReleaseTarballSpecification, noDownload bool) (cargo.BOSHReleaseTarballLock, error) {
	var foundReleaseLock []cargo.BOSHReleaseTarballLock
	for _, src := range list {
		rel, err := src.FindReleaseVersion(requirement, noDownload)
		if err != nil {
			if !IsErrNotFound(err) {
				return cargo.BOSHReleaseTarballLock{}, scopedError(src.Configuration().ID, err)
			}
			continue
		}
		foundReleaseLock = append(foundReleaseLock, rel)
	}
	if len(foundReleaseLock) == 0 {
		return cargo.BOSHReleaseTarballLock{}, ErrNotFound
	}
	highestLock := foundReleaseLock[0]
	highestVersion, err := highestLock.ParseVersion()
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, fmt.Errorf("failed to parse version from release source: %w", err)
	}
	for _, rel := range foundReleaseLock[1:] {
		newVersion, err := semver.NewVersion(rel.Version)
		if err != nil {
			return cargo.BOSHReleaseTarballLock{}, fmt.Errorf("failed to parse version from release source: %w", err)
		}
		if highestVersion.LessThan(newVersion) {
			highestVersion = newVersion
			highestLock = rel
		}
	}
	return highestLock, nil
}

func (list ReleaseSourceList) FindByID(id string) (ReleaseSource, error) {
	var correctSrc ReleaseSource
	for _, src := range list {
		if src.Configuration().ID == id {
			correctSrc = src
			break
		}
	}

	if correctSrc == nil {
		ids := make([]string, 0, len(list))
		for _, src := range list {
			ids = append(ids, src.Configuration().ID)
		}
		return nil, fmt.Errorf("couldn't find a release source with ID %q. Available choices: %q", id, ids)
	}

	return correctSrc, nil
}

func scopedError(sourceID string, err error) error {
	return fmt.Errorf("error from release source %q: %w", sourceID, err)
}
