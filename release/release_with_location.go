package release

type unhomedRelease interface {
	Satisfies(set ReleaseRequirement) bool
	StandardizedFilename() string
	ReleaseID() ReleaseID
}

//go:generate counterfeiter -o ./fakes/release_with_location.go --fake-name ReleaseWithLocation . ReleaseWithLocation
type ReleaseWithLocation interface {
	unhomedRelease
	LocalPath() string
	RemotePath() string
	WithLocalPath(string) ReleaseWithLocation
	WithRemotePath(string) ReleaseWithLocation
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

func (r releaseWithLocation) WithRemotePath(remotePath string) ReleaseWithLocation {
	r.remotePath = remotePath
	return r
}

func (r releaseWithLocation) WithLocalPath(localPath string) ReleaseWithLocation {
	r.localPath = localPath
	return r
}

