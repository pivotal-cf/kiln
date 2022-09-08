package component

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/Masterminds/semver"
)

type ReleaseSources struct {
	List []ReleaseSource
}

func NewReleaseSources(sources ...ReleaseSource) *ReleaseSources {
	return &ReleaseSources{List: sources}
}

var _ MultiReleaseSource = &ReleaseSources{}

func (sources *ReleaseSources) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw []*EncodedReleaseSource
	err := unmarshal(&raw)
	if err != nil {
		return err
	}
	for _, r := range raw {
		sources.List = append(sources.List, r.ReleaseSource)
	}
	return nil
}

func (sources *ReleaseSources) MarshalYAML() (interface{}, error) {
	enc := make([]*EncodedReleaseSource, 0, len(sources.List))
	for _, source := range sources.List {
		enc = append(enc, &EncodedReleaseSource{ReleaseSource: source})
	}
	return enc, nil
}

func (sources *ReleaseSources) ConfigurationErrors() []error {
	var result []error
	err := sources.checkIfDuplicateIDs()
	if err != nil {
		result = append(result, err)
	}
	for _, source := range sources.List {
		result = append(result, source.ConfigurationErrors()...)
	}
	return result
}

func (sources *ReleaseSources) checkIfDuplicateIDs() error {
	indexOfID := make(map[string]int)
	for index, rs := range sources.List {
		id := rs.ID()
		previousIndex, seen := indexOfID[id]
		if seen {
			return fmt.Errorf(fmt.Sprintf(`release_sources must have unique IDs; items at index %d and %d both have ID %q`, previousIndex, index, id))
		}
		indexOfID[id] = index
	}
	return nil
}

func (sources *ReleaseSources) Filter(allowOnlyPublishable bool) *ReleaseSources {
	var filtered ReleaseSources
	for _, source := range sources.List {
		if allowOnlyPublishable && !source.IsPublishable() {
			continue
		}
		filtered.List = append(filtered.List, source)
	}
	return &filtered
}

func (sources *ReleaseSources) FindReleaseUploader(sourceID string) (ReleaseUploader, error) {
	var (
		uploader     ReleaseUploader
		availableIDs []string
	)
	for _, src := range sources.List {
		u, ok := src.(ReleaseUploader)
		if !ok {
			continue
		}
		availableIDs = append(availableIDs, src.ID())
		if src.ID() == sourceID {
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

func wrapLogger(src ReleaseSource, logger *log.Logger) *log.Logger {
	sourceLogger := NewReleaseSourceLogger(src, logger.Writer())
	sourceLogger.SetPrefix(logger.Prefix() + sourceLogger.Prefix())
	return sourceLogger
}

func (sources *ReleaseSources) GetMatchedRelease(ctx context.Context, logger *log.Logger, requirement Spec) (Lock, error) {
	for _, src := range sources.List {
		rel, err := src.GetMatchedRelease(ctx, wrapLogger(src, logger), requirement)
		if err != nil {
			if IsErrNotFound(err) {
				continue
			}
			return Lock{}, scopedError(src.ID(), err)
		}
		return rel, nil
	}
	return Lock{}, ErrNotFound
}

func (sources *ReleaseSources) SetDownloadThreads(n int) {
	if n == 0 || sources == nil {
		return
	}
	for i, rs := range sources.List {
		s3rs, ok := rs.(*S3ReleaseSource)
		if ok && s3rs != nil {
			s3rs.DownloadThreads = n
			sources.List[i] = s3rs
		}
	}
}

const (
	logLineDownload = "downloading %s from %s release source %s"
)

func (sources *ReleaseSources) DownloadRelease(ctx context.Context, logger *log.Logger, releaseDir string, remoteRelease Lock) (Local, error) {
	src, err := sources.FindByID(remoteRelease.RemoteSource)
	if err != nil {
		return Local{}, err
	}

	localRelease, err := src.DownloadRelease(ctx, wrapLogger(src, logger), releaseDir, remoteRelease)
	if err != nil {
		return Local{}, scopedError(src.ID(), err)
	}

	return localRelease, nil
}

func (sources *ReleaseSources) FindReleaseVersion(ctx context.Context, logger *log.Logger, requirement Spec) (Lock, error) {
	var foundReleaseLock []Lock
	for _, src := range sources.List {
		rel, err := src.FindReleaseVersion(ctx, wrapLogger(src, logger), requirement)
		if err != nil {
			if !IsErrNotFound(err) {
				return Lock{}, scopedError(src.ID(), err)
			}
			continue
		}
		foundReleaseLock = append(foundReleaseLock, rel)
	}
	if len(foundReleaseLock) == 0 {
		return Lock{}, ErrNotFound
	}
	highestLock := foundReleaseLock[0]
	highestVersion, err := highestLock.ParseVersion()
	if err != nil {
		return Lock{}, fmt.Errorf("failed to parse version from release source: %w", err)
	}
	for _, rel := range foundReleaseLock[1:] {
		newVersion, err := semver.NewVersion(rel.Version)
		if err != nil {
			return Lock{}, fmt.Errorf("failed to parse version from release source: %w", err)
		}
		if highestVersion.LessThan(newVersion) {
			highestVersion = newVersion
			highestLock = rel
		}
	}
	return highestLock, nil
}

func (sources *ReleaseSources) FindByID(id string) (ReleaseSource, error) {
	var correctSrc ReleaseSource
	for _, src := range sources.List {
		if src.ID() == id {
			correctSrc = src
			break
		}
	}

	if correctSrc == nil {
		return nil, fmt.Errorf("couldn't find a release source with ID %q. Available choices: %q", id, sources.IDs())
	}

	return correctSrc, nil
}

func (sources *ReleaseSources) Add(rs ReleaseSource) {
	sources.List = append(sources.List, rs)
}

func (sources *ReleaseSources) IDs() []string {
	ids := make([]string, 0, len(sources.List))
	for _, src := range sources.List {
		ids = append(ids, src.ID())
	}
	return ids
}
