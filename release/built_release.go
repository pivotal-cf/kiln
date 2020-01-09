package release

type builtRelease ReleaseID

func NewBuiltRelease(id ReleaseID, localPath string) satisfiableLocalRelease {
	return satisfiableLocalRelease{unhomedRelease: builtRelease(id), localPath: localPath}
}

func (br builtRelease) Satisfies(rr ReleaseRequirement) bool {
	return br.Name == rr.Name &&
		br.Version == rr.Version
}

func (br builtRelease) ReleaseID() ReleaseID {
	return ReleaseID(br)
}
