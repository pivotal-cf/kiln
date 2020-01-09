package release

type AdditionalConstraint interface {
	Satisfies(set ReleaseRequirement) bool
}

type SatisfiableLocalReleaseSet map[ReleaseID]SatisfiableLocalRelease

type SatisfiableLocalRelease struct {
	LocalRelease
	AdditionalConstraint
}

func (r SatisfiableLocalRelease) Satisfies(rr ReleaseRequirement) bool {
	if r.AdditionalConstraint == nil {
		r.AdditionalConstraint = noConstraint{}
	}
	return r.ReleaseID.Name == rr.Name &&
		r.ReleaseID.Version == rr.Version &&
		r.AdditionalConstraint.Satisfies(rr)
}

type noConstraint struct {}

func (noConstraint) Satisfies(ReleaseRequirement) bool {
	return true
}

func NewBuiltRelease(id ReleaseID, localPath string) SatisfiableLocalRelease {
	return SatisfiableLocalRelease{
		LocalRelease: LocalRelease{ReleaseID: id, LocalPath: localPath},
	}
}

func NewCompiledRelease(id ReleaseID, stemcellOS, stemcellVersion, localPath string) SatisfiableLocalRelease {
	return SatisfiableLocalRelease{
		LocalRelease: LocalRelease{ReleaseID: id, LocalPath: localPath},
		AdditionalConstraint: StemcellConstraint{
			StemcellOS:      stemcellOS,
			StemcellVersion: stemcellVersion,
		},
	}
}

type StemcellConstraint struct {
	StemcellOS      string
	StemcellVersion string
}

func (cr StemcellConstraint) Satisfies(rr ReleaseRequirement) bool {
	return cr.StemcellOS == rr.StemcellOS &&
		cr.StemcellVersion == rr.StemcellVersion
}
