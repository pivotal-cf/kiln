package release

type compiledRelease struct {
	builtRelease
	StemcellOS      string
	StemcellVersion string
}

func NewCompiledRelease(id ReleaseID, stemcellOS, stemcellVersion, localPath string) satisfiableLocalRelease {
	return satisfiableLocalRelease{
		unhomedRelease: compiledRelease{
			builtRelease:    builtRelease(id),
			StemcellOS:      stemcellOS,
			StemcellVersion: stemcellVersion,
		},
		localPath: localPath,
	}
}

func (cr compiledRelease) Satisfies(rr ReleaseRequirement) bool {
	return cr.builtRelease.Satisfies(rr) &&
		cr.StemcellOS == rr.StemcellOS &&
		cr.StemcellVersion == rr.StemcellVersion
}
