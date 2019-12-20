package release

type ReleaseWithLocationSet map[ReleaseID]ReleaseWithLocation

func (rs ReleaseWithLocationSet) ReleaseIDs() []ReleaseID {
	result := make([]ReleaseID, 0, len(rs))
	for rID := range rs {
		result = append(result, rID)
	}
	return result
}

func (rs ReleaseWithLocationSet) With(toAdd ReleaseWithLocationSet) ReleaseWithLocationSet {
	result := rs.copy()
	for releaseID, release := range toAdd {
		result[releaseID] = release
	}
	return result
}

func (rs ReleaseWithLocationSet) copy() ReleaseWithLocationSet {
	dup := make(ReleaseWithLocationSet)
	for releaseID, release := range rs {
		dup[releaseID] = release
	}
	return dup
}
