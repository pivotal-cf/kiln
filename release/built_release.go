package release

type builtRelease ReleaseID

func NewBuiltRelease(id ReleaseID) releaseWithLocation {
	return releaseWithLocation{unhomedRelease: builtRelease(id)}
}

func (br builtRelease) Satisfies(rr ReleaseRequirement) bool {
	return br.Name == rr.Name &&
		br.Version == rr.Version
}

func (br builtRelease) ReleaseID() ReleaseID {
	return ReleaseID(br)
}
