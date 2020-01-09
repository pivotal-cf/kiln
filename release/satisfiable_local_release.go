package release

type unhomedRelease interface {
	Satisfies(set ReleaseRequirement) bool
	ReleaseID() ReleaseID
}

type SatisfiableLocalRelease interface {
	unhomedRelease
	LocalPath() string
}

type SatisfiableLocalReleaseSet map[ReleaseID]SatisfiableLocalRelease

type satisfiableLocalRelease struct {
	unhomedRelease
	localPath      string
}

func (r satisfiableLocalRelease) LocalPath() string {
	return r.localPath
}
