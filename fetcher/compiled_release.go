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

func (cr CompiledRelease) IsBuiltRelease() bool {
	return cr.StemcellOS == "" && cr.StemcellVersion == ""
}
