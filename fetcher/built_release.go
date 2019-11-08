package fetcher

type BuiltRelease struct {
	ID   ReleaseID
	Path string
}

func (br BuiltRelease) DownloadString() string {
	return br.Path
}
