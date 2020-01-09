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

func (rrs ReleaseRequirementSet) Partition(other []SatisfyingLocalRelease) (intersection LocalReleaseSet, missing ReleaseRequirementSet, extra LocalReleaseSet) {
	intersection = make(LocalReleaseSet)
	missing = rrs.copy()
	extra = make(LocalReleaseSet)

	for _, rel := range other {
		req, ok := rrs[rel.ReleaseID]
		if ok  && rel.Satisfies(req) {
			intersection[rel.ReleaseID] = rel.LocalRelease
			delete(missing, rel.ReleaseID)
		} else {
			extra[rel.ReleaseID] = rel.LocalRelease
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
