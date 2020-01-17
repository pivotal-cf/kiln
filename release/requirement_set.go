package release

import (
	"github.com/pivotal-cf/kiln/internal/cargo"
)

type RequirementSet map[ID]Requirement

func NewRequirementSet(kilnfileLock cargo.KilnfileLock) RequirementSet {
	set := make(RequirementSet)
	stemcell := kilnfileLock.Stemcell
	for _, release := range kilnfileLock.Releases {
		requirement := newRequirement(release, stemcell)
		set[requirement.releaseID()] = requirement
	}
	return set
}

func (rrs RequirementSet) Partition(other []LocalSatisfying) (intersection []Local, missing RequirementSet, extra []Local) {
	missing = rrs.copy()

	for _, rel := range other {
		req, ok := rrs[rel.ID]
		if ok && rel.Satisfies(req) {
			intersection = append(intersection, rel.Local)
			delete(missing, rel.ID)
		} else {
			extra = append(extra, rel.Local)
		}
	}

	return intersection, missing, extra
}

func (rrs RequirementSet) copy() RequirementSet {
	dup := make(RequirementSet)
	for releaseID, release := range rrs {
		dup[releaseID] = release
	}
	return dup
}
