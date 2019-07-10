package fetcher

import (
	"github.com/pivotal-cf/kiln/internal/cargo"
)

type CompiledRelease struct {
	Name            string
	Version         string
	StemcellOS      string
	StemcellVersion string
}

type CompiledReleaseSet map[CompiledRelease]string

func newCompiledRelease(release cargo.Release, stemcell cargo.Stemcell) CompiledRelease {
	return CompiledRelease{
		Name:            release.Name,
		Version:         release.Version,
		StemcellOS:      stemcell.OS,
		StemcellVersion: stemcell.Version,
	}
}

func NewCompiledReleaseSet(assetsLock cargo.AssetsLock) CompiledReleaseSet {
	set := make(CompiledReleaseSet)
	stemcell := assetsLock.Stemcell
	for _, release := range assetsLock.Releases {
		compiledRelease := newCompiledRelease(release, stemcell)
		set[compiledRelease] = ""
	}
	return set
}

func (set CompiledReleaseSet) Contains(release CompiledRelease) (CompiledRelease, bool) {
	_, ok := set[release]
	if ok {
		return release, ok
	} else {
		if isBuiltRelease(release) {
			for key := range set {
				if release.Name == key.Name && release.Version == key.Version {
					return key, true
				}
			}
		}
		return CompiledRelease{}, ok
	}
}

// Feature Envy
func isBuiltRelease(release CompiledRelease) bool {
	return release.StemcellOS == "" && release.StemcellVersion == ""
}


func (crs CompiledReleaseSet) copy() CompiledReleaseSet {
	dup := make(CompiledReleaseSet)
	for release, path := range crs {
		dup[release] = path
	}
	return dup
}

func (crs CompiledReleaseSet) With(toAdd CompiledReleaseSet) CompiledReleaseSet {
	result := crs.copy()
	for release, path := range toAdd {
		result[release] = path
	}
	return result
}

func (crs CompiledReleaseSet) Without(other CompiledReleaseSet) CompiledReleaseSet {
	result := crs.copy()
	for release := range result {
		release, ok := other.Contains(release)
		if ok {
			delete(result, release)
		}
	}
	return result
}

func (source CompiledReleaseSet) TransferElements(toAdd, dest CompiledReleaseSet) (CompiledReleaseSet, CompiledReleaseSet) {
	sor := source.copy()
	des := dest.copy()

	for release, path := range toAdd {
		match, ok := sor.Contains(release)
		if ok {
			delete(sor, match)
			des[release] = path
		}
	}

	return sor, des
}
