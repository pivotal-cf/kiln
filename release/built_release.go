package release

import (
	"fmt"
)

type BuiltRelease struct {
	ID         ReleaseID
	localPath  string
	remotePath string
}

func NewBuiltRelease(id ReleaseID, localPath, remotePath string) BuiltRelease {
	return BuiltRelease{
		ID:         id,
		localPath:  localPath,
		remotePath: remotePath,
	}
}

func (br BuiltRelease) RemotePath() string {
	return br.remotePath
}

func (br BuiltRelease) StandardizedFilename() string {
	return fmt.Sprintf("%s-%s.tgz", br.ID.Name, br.ID.Version)
}

func (br BuiltRelease) LocalPath() string {
	return br.localPath
}

func (br BuiltRelease) Satisfies(rr ReleaseRequirement) bool {
	return br.ID.Name == rr.Name &&
		br.ID.Version == rr.Version
}

func (br BuiltRelease) ReleaseID() ReleaseID {
	return br.ID
}

func (br BuiltRelease) AsLocal(path string) LocalRelease {
	br.localPath = path
	return br
}
