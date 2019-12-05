package fetcher

import "github.com/pivotal-cf/kiln/internal/cargo"

type ReleaseRequirementSet map[ReleaseID]ReleaseRequirement

func NewReleaseRequirementSet(kilnfileLock cargo.KilnfileLock) ReleaseRequirementSet {
	set := make(ReleaseRequirementSet)
	stemcell := kilnfileLock.Stemcell
	for _, release := range kilnfileLock.Releases {
		requirement := newReleaseRequirement(release, stemcell)
		set[requirement.releaseID()] = requirement
	}
	return set
}

func (rrs ReleaseRequirementSet) Partition(other LocalReleaseSet) (intersection LocalReleaseSet, missing ReleaseRequirementSet, extra LocalReleaseSet) {
	intersection = make(LocalReleaseSet)
	missing = make(ReleaseRequirementSet)
	extra = other.copy()

	for rID, requirement := range rrs {
		otherRelease, ok := other[rID]
		if ok && otherRelease.Satisfies(requirement) {
			intersection[rID] = otherRelease
			delete(extra, rID)
		} else {
			missing[rID] = requirement
		}
	}
	return intersection, missing, extra
}

func (rrs ReleaseRequirementSet) WithoutReleases(toRemove []ReleaseID) ReleaseRequirementSet {
	result := rrs.copy()

	for _, rID := range toRemove {
		delete(result, rID)
	}

	return result
}

func (rrs ReleaseRequirementSet) copy() ReleaseRequirementSet {
	dup := make(ReleaseRequirementSet)
	for releaseID, release := range rrs {
		dup[releaseID] = release
	}
	return dup
}

func newReleaseRequirement(release cargo.Release, stemcell cargo.Stemcell) ReleaseRequirement {
	return ReleaseRequirement{
		Name:            release.Name,
		Version:         release.Version,
		StemcellOS:      stemcell.OS,
		StemcellVersion: stemcell.Version,
	}
}

type ReleaseRequirement struct {
	Name, Version, StemcellOS, StemcellVersion string
}

func (rr ReleaseRequirement) releaseID() ReleaseID {
	return ReleaseID{Name: rr.Name, Version: rr.Version}
}
