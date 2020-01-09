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

func NewBuiltRelease(id ReleaseID, localPath string) SatisfiableLocalRelease {
	return SatisfiableLocalRelease{
		LocalRelease: LocalRelease{ReleaseID: id, LocalPath: localPath},
	}
}

type stemcellConstraints struct {
	StemcellOS      string
	StemcellVersion string
}

func NewCompiledRelease(id ReleaseID, stemcellOS, stemcellVersion, localPath string) SatisfiableLocalRelease {
	return SatisfiableLocalRelease{
		LocalRelease: LocalRelease{ReleaseID: id, LocalPath: localPath},
		additionalConstraints: stemcellConstraints{
			StemcellOS:      stemcellOS,
			StemcellVersion: stemcellVersion,
		},
	}
}

func (cr stemcellConstraints) Satisfies(rr ReleaseRequirement) bool {
	return cr.StemcellOS == rr.StemcellOS &&
		cr.StemcellVersion == rr.StemcellVersion
}
