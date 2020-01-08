package release

type LocalRelease struct {
	ReleaseID ReleaseID
	LocalPath string
}

type LocalReleaseSet map[ReleaseID]LocalRelease

func (lrs LocalReleaseSet) With(toAdd LocalReleaseSet) LocalReleaseSet {
	result := lrs.copy()
	for releaseID, release := range toAdd {
		result[releaseID] = release
	}
	return result
}

func (lrs LocalReleaseSet) copy() LocalReleaseSet {
	dup := make(LocalReleaseSet)
	for releaseID, release := range lrs {
		dup[releaseID] = release
	}
	return dup
}

func (lrs LocalReleaseSet) ReleaseIDs() []ReleaseID {
	result := make([]ReleaseID, 0, len(lrs))
	for rID := range lrs {
		result = append(result, rID)
	}
	return result
}
