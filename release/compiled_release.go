package release

type stemcellConstraints struct {
	StemcellOS      string
	StemcellVersion string
}

func NewCompiledRelease(id ReleaseID, stemcellOS, stemcellVersion, localPath string) satisfiableLocalRelease {
	return satisfiableLocalRelease{
		releaseID: id,
		localPath: localPath,
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
