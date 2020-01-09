package release

type unhomedRelease interface {
	Satisfies(set ReleaseRequirement) bool
	ReleaseID() ReleaseID
}

type ReleaseWithLocation interface {
	unhomedRelease
	LocalPath() string
	WithLocalPath(string) ReleaseWithLocation
}

type ReleaseWithLocationSet map[ReleaseID]ReleaseWithLocation

type releaseWithLocation struct {
	unhomedRelease
	localPath      string
}

func (r releaseWithLocation) LocalPath() string {
	return r.localPath
}

func (r releaseWithLocation) WithLocalPath(localPath string) ReleaseWithLocation {
	r.localPath = localPath
	return r
}
