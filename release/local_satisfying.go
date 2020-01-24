package release

type ExtraConstraint interface {
	Satisfies(set Requirement) bool
}

type LocalSatisfying struct {
	Local
	ExtraConstraint
}

func (r LocalSatisfying) Satisfies(rr Requirement) bool {
	if r.ExtraConstraint == nil {
		r.ExtraConstraint = noConstraint{}
	}
	return r.Name == rr.Name &&
		r.Version == rr.Version &&
		r.ExtraConstraint.Satisfies(rr)
}

type noConstraint struct{}

func (noConstraint) Satisfies(Requirement) bool {
	return true
}

func NewLocalBuilt(id ID, localPath string) LocalSatisfying {
	return LocalSatisfying{
		Local: Local{ID: id, LocalPath: localPath},
	}
}

func NewLocalCompiled(id ID, stemcellOS, stemcellVersion, localPath string) LocalSatisfying {
	return LocalSatisfying{
		Local: Local{ID: id, LocalPath: localPath},
		ExtraConstraint: StemcellConstraint{
			StemcellOS:      stemcellOS,
			StemcellVersion: stemcellVersion,
		},
	}
}

type StemcellConstraint struct {
	StemcellOS      string
	StemcellVersion string
}

func (cr StemcellConstraint) Satisfies(rr Requirement) bool {
	return cr.StemcellOS == rr.StemcellOS &&
		cr.StemcellVersion == rr.StemcellVersion
}
