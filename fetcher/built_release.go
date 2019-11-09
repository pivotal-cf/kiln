package fetcher

import "fmt"

type BuiltRelease struct {
	ID   ReleaseID
	Path string
}

func (br BuiltRelease) RemotePath() string {
	return br.Path
}

func (br BuiltRelease) StandardizedFilename() string {
	return fmt.Sprintf("%s-%s.tgz", br.ID.Name, br.ID.Version)
}

func (br BuiltRelease) LocalPath() string {
	return br.Path
}

func (br BuiltRelease) Satisfies(rr ReleaseRequirement) bool {
	return br.ID.Name == rr.Name &&
		br.ID.Version == rr.Version
}

func (br BuiltRelease) ReleaseID() ReleaseID {
	return br.ID
}

func (br BuiltRelease) AsLocal(path string) LocalRelease {
	br.Path = path
	return br
}
