package release

func NewBuiltRelease(id ReleaseID, localPath string) SatisfiableLocalRelease {
	return SatisfiableLocalRelease{
		LocalRelease: LocalRelease{ReleaseID: id, LocalPath: localPath},
	}
}
