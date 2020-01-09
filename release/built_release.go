package release

func NewBuiltRelease(id ReleaseID, localPath string) satisfiableLocalRelease {
	return satisfiableLocalRelease{
		releaseID: id,
		localPath: localPath,
	}
}
