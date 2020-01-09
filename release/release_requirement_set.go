package release

import (
	"github.com/pivotal-cf/kiln/internal/cargo"
)

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

func (rrs ReleaseRequirementSet) Partition(other SatisfiableLocalReleaseSet) (intersection LocalReleaseSet, missing ReleaseRequirementSet, extra LocalReleaseSet) {
	intersection = make(LocalReleaseSet)
	missing = make(ReleaseRequirementSet)
	extra = make(LocalReleaseSet)

	for rID, rel := range other {
		extra[rID] = LocalRelease{
			ReleaseID: rel.ReleaseID(),
			LocalPath: rel.LocalPath(),
		}
	}

	for rID, requirement := range rrs {
		otherRelease, ok := other[rID]
		if ok && otherRelease.Satisfies(requirement) {
			intersection[rID] = extra[rID]
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
