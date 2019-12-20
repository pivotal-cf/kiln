package release

type unhomedRelease interface {
	Satisfies(set ReleaseRequirement) bool
	StandardizedFilename() string
	ReleaseID() ReleaseID
}

type ReleaseWithLocation interface {
	unhomedRelease
	RemotePath() string
	LocalPath() string
	AsLocal(string) LocalRelease
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
