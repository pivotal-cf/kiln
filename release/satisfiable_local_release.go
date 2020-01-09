package release

type additionalConstraints interface {
	Satisfies(set ReleaseRequirement) bool
}

type SatisfiableLocalRelease interface {
	additionalConstraints
	ReleaseID() ReleaseID
	LocalPath() string
	LocalRelease() LocalRelease
}

type SatisfiableLocalReleaseSet map[ReleaseID]SatisfiableLocalRelease

type satisfiableLocalRelease struct {
	additionalConstraints
	releaseID ReleaseID
	localPath      string
}

func (r satisfiableLocalRelease) ReleaseID() ReleaseID {
	return r.releaseID
}

func (r satisfiableLocalRelease) LocalPath() string {
	return r.localPath
}

func (r satisfiableLocalRelease) Satisfies(rr ReleaseRequirement) bool {
	if r.releaseID.Name == rr.Name && r.releaseID.Version == rr.Version {
		if r.additionalConstraints == nil {
			return true
		} else {
			return r.additionalConstraints.Satisfies(rr)
		}
	}
	return false
}

func (r satisfiableLocalRelease) LocalRelease() LocalRelease {
	return LocalRelease{ReleaseID: r.releaseID, LocalPath: r.localPath}
}
