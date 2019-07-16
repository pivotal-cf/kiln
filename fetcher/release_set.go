package fetcher

import (
	"github.com/pivotal-cf/kiln/internal/cargo"
)

type ReleaseID struct {
	Name, Version string
}

type BuiltRelease struct {
	ID   ReleaseID
	Path string
}

type CompiledRelease struct {
	ID              ReleaseID
	StemcellOS      string
	StemcellVersion string
	Path            string
}

type ReleaseInfoDownloader interface {
	DownloadString() string
}

func (br BuiltRelease) DownloadString() string {
	return br.Path
}
func (cr CompiledRelease) DownloadString() string {
	return cr.Path
}

type ReleaseSet map[ReleaseID]ReleaseInfoDownloader

//func init() {
//	all_the_releases := make(ReleaseSet)
//
//	compiled_release := CompiledRelease{ID: ReleaseID{"foo", "0.1.0"}, StemcellOS: "plan9", StemcellVersion: "0.0.1", Path: "/bla"}
//
//	built_release := BuiltRelease{ID: ReleaseID{"poo", "1.1.0"}, Path: "/paa"}
//
//	all_the_releases[compiled_release.ID] = compiled_release
//	all_the_releases[built_release.ID] = built_release
//}

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

func NewReleaseSet(assetsLock cargo.AssetsLock) ReleaseSet {
	set := make(ReleaseSet)
	stemcell := assetsLock.Stemcell
	for _, release := range assetsLock.Releases {
		compiledRelease := newCompiledRelease(release, stemcell)
		set[compiledRelease.ID] = compiledRelease
	}
	return set
}

func (set ReleaseSet) Contains(releaseID ReleaseID) (ReleaseID, bool) {
	_, ok := set[releaseID]
	if ok {
		return releaseID, ok
	} else {
		return ReleaseID{}, ok
	}

	//	if release.IsBuiltRelease() {
	//		for key := range set {
	//			if release.Name == key.Name && release.Version == key.Version {
	//				return key, true
	//			}
	//		}
	//	}
	//	return CompiledRelease{}, ok
	//}
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
		releaseID, ok := other.Contains(releaseID)
		if ok {
			delete(result, releaseID)
		}
	}
	return result
}

func (source ReleaseSet) TransferElements(toAdd, dest ReleaseSet) (ReleaseSet, ReleaseSet) {
	sor := source.copy()
	des := dest.copy()

	for releaseID, release := range toAdd {
		match, ok := sor.Contains(releaseID)
		if ok {
			delete(sor, match)
			des[releaseID] = release
		}
	}

	return sor, des
}
