package fetcher

type CompiledRelease struct {
	ID              ReleaseID
	StemcellOS      string
	StemcellVersion string
	Path            string
}

func (cr CompiledRelease) DownloadString() string {
	return cr.Path
}

func (cr CompiledRelease) Satisfies(rr ReleaseRequirement) bool {
	return cr.ID.Name == rr.Name &&
		cr.ID.Version == rr.Version &&
		cr.StemcellOS == rr.StemcellOS &&
		cr.StemcellVersion == rr.StemcellVersion
}
