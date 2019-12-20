package release

type unhomedRelease interface {
	Satisfies(set ReleaseRequirement) bool
	StandardizedFilename() string
	ReleaseID() ReleaseID
}

//go:generate counterfeiter -o ./fakes/release_with_location.go --fake-name ReleaseWithLocation . ReleaseWithLocation
type ReleaseWithLocation interface {
	unhomedRelease
	AsLocal(string) LocalRelease
	LocalPath() string
	RemotePath() string
	WithLocalPath(string) ReleaseWithLocation
}

type releaseWithLocation struct {
	unhomedRelease
	localPath string
	remotePath string
}

func (r releaseWithLocation) RemotePath() string {
	return r.remotePath
}
func (r releaseWithLocation) LocalPath() string {
	return r.localPath
}

func (r releaseWithLocation) AsLocal(path string) LocalRelease {
	r.localPath = path
	return r
}

func (r releaseWithLocation) WithLocalPath(path string) ReleaseWithLocation {
	r.localPath = path
	return r
}

type ReleaseWithLocationSet map[ReleaseID]ReleaseWithLocation

func (rs ReleaseWithLocationSet) ReleaseIDs() []ReleaseID {
	result := make([]ReleaseID, 0, len(rs))
	for rID := range rs {
		result = append(result, rID)
	}
	return result
}
