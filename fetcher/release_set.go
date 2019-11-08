package fetcher

import (
	"github.com/pivotal-cf/kiln/internal/cargo"
)

type ReleaseID struct {
	Name, Version string
}

type ReleaseInfo interface {
	DownloadString() string
}

type ReleaseSet map[ReleaseID]ReleaseInfo

func newCompiledRelease(release cargo.Release, stemcell cargo.Stemcell) CompiledRelease {
	return CompiledRelease{
		ID: ReleaseID{
			Name:    release.Name,
			Version: release.Version,
		},
		StemcellOS:      stemcell.OS,
		StemcellVersion: stemcell.Version,
		Path:            "",
	}
}

func NewReleaseSet(kilnfileLock cargo.KilnfileLock) ReleaseSet {
	set := make(ReleaseSet)
	stemcell := kilnfileLock.Stemcell
	for _, release := range kilnfileLock.Releases {
		compiledRelease := newCompiledRelease(release, stemcell)
		set[compiledRelease.ID] = compiledRelease
	}
	return set
}

func (rel CompiledRelease) IsBuiltRelease() bool {
	return rel.StemcellOS == "" && rel.StemcellVersion == ""
}

func (crs ReleaseSet) copy() ReleaseSet {
	dup := make(ReleaseSet)
	for releaseID, release := range crs {
		dup[releaseID] = release
	}
	return dup
}

func (crs ReleaseSet) With(toAdd ReleaseSet) ReleaseSet {
	result := crs.copy()
	for releaseID, release := range toAdd {
		result[releaseID] = release
	}
	return result
}

func (crs ReleaseSet) Without(other ReleaseSet) ReleaseSet {
	result := crs.copy()
	for releaseID := range result {
		if _, ok := other[releaseID]; ok {
			delete(result, releaseID)
		}
	}
	return result
}

func (source ReleaseSet) TransferElements(toAdd, dest ReleaseSet) (ReleaseSet, ReleaseSet) {
	sor := source.copy()
	des := dest.copy()

	for releaseID, release := range toAdd {
		if _, ok := sor[releaseID]; ok {
			delete(sor, releaseID)
			des[releaseID] = release
		}
	}

	return sor, des
}
