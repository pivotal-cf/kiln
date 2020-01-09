package release

type additionalConstraints interface {
	Satisfies(set ReleaseRequirement) bool
}

type SatisfiableLocalReleaseSet map[ReleaseID]SatisfiableLocalRelease

type SatisfiableLocalRelease struct {
	LocalRelease
	additionalConstraints
}

func (r SatisfiableLocalRelease) Satisfies(rr ReleaseRequirement) bool {
	if r.ReleaseID.Name == rr.Name && r.ReleaseID.Version == rr.Version {
		if r.additionalConstraints == nil {
			return true
		} else {
			return r.additionalConstraints.Satisfies(rr)
		}
	}
	return false
}
