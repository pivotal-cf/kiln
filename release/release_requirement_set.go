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

func (rrs ReleaseRequirementSet) Partition(other []SatisfyingLocalRelease) (intersection []LocalRelease, missing ReleaseRequirementSet, extra []LocalRelease) {
	missing = rrs.copy()

	for _, rel := range other {
		req, ok := rrs[rel.ReleaseID]
		if ok && rel.Satisfies(req) {
			intersection = append(intersection, rel.LocalRelease)
			delete(missing, rel.ReleaseID)
		} else {
			extra = append(extra, rel.LocalRelease)
		}
	}

	return intersection, missing, extra
}

func (rrs ReleaseRequirementSet) copy() ReleaseRequirementSet {
	dup := make(ReleaseRequirementSet)
	for releaseID, release := range rrs {
		dup[releaseID] = release
	}
	return dup
}
