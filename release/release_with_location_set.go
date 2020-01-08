package release

type ReleaseWithLocationSet map[ReleaseID]ReleaseWithLocation

func (rs ReleaseWithLocationSet) copy() ReleaseWithLocationSet {
	dup := make(ReleaseWithLocationSet)
	for releaseID, release := range rs {
		dup[releaseID] = release
	}
	return dup
}

func (rs ReleaseWithLocationSet) AsLocalReleaseSet() LocalReleaseSet {
	set := make(LocalReleaseSet)
	for k, v := range rs {
		set[k] = LocalRelease{ReleaseID: v.ReleaseID(), LocalPath: v.LocalPath()}
	}
	return set
}
