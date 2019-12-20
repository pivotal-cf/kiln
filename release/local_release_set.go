package release

type LocalReleaseSet map[ReleaseID]LocalRelease

func (rs LocalReleaseSet) With(toAdd LocalReleaseSet) LocalReleaseSet {
	result := rs.copy()
	for releaseID, release := range toAdd {
		result[releaseID] = release
	}
	return result
}

func (rs LocalReleaseSet) ReleaseIDs() []ReleaseID {
	result := make([]ReleaseID, 0, len(rs))
	for rID := range rs {
		result = append(result, rID)
	}
	return result
}

func (rs LocalReleaseSet) LocalReleases() []LocalRelease {
	result := make([]LocalRelease, 0, len(rs))
	for _, rInfo := range rs {
		result = append(result, rInfo)
	}
	return result
}

func (rs LocalReleaseSet) copy() LocalReleaseSet {
	dup := make(LocalReleaseSet)
	for releaseID, release := range rs {
		dup[releaseID] = release
	}
	return dup
}
