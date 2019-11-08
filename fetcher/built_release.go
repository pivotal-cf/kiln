package fetcher

type BuiltRelease struct {
	ID   ReleaseID
	Path string
}

func (br BuiltRelease) DownloadString() string {
	return br.Path
}

func (br BuiltRelease) Satisfies(rr ReleaseRequirement) bool {
	return br.ID.Name == rr.Name &&
		br.ID.Version == rr.Version
}
