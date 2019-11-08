package fetcher

type ReleaseID struct {
	Name, Version string
}

//go:generate counterfeiter -o ./fakes/release_info.go --fake-name ReleaseInfo . ReleaseInfo
type ReleaseInfo interface {
	DownloadString() string
	Satisfies(set ReleaseRequirement) bool
}

type ReleaseSet map[ReleaseID]ReleaseInfo

func (rs ReleaseSet) With(toAdd ReleaseSet) ReleaseSet {
	result := rs.copy()
	for releaseID, release := range toAdd {
		result[releaseID] = release
	}
	return result
}

func (rs ReleaseSet) ReleaseIDs() []ReleaseID {
	result := make([]ReleaseID, 0, len(rs))
	for rID := range rs {
		result = append(result, rID)
	}
	return result
}

func (rs ReleaseSet) copy() ReleaseSet {
	dup := make(ReleaseSet)
	for releaseID, release := range rs {
		dup[releaseID] = release
	}
	return dup
}
