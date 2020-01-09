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
	AsLocalRelease(string) LocalRelease
	WithRemote(remoteSourceID string, remotePath string) ReleaseWithLocation
	ReleaseSourceID() string
	AsRemoteRelease() RemoteRelease
}

type releaseWithLocation struct {
	unhomedRelease
	localPath      string
	remoteSourceID string
	remotePath     string
}

func (r releaseWithLocation) RemotePath() string {
	return r.remotePath
}
func (r releaseWithLocation) LocalPath() string {
	return r.localPath
}

func (r releaseWithLocation) WithRemote(remoteSourceID string, remotePath string) ReleaseWithLocation {
	r.remoteSourceID = remoteSourceID
	r.remotePath = remotePath
	return r
}

func (r releaseWithLocation) WithLocalPath(localPath string) ReleaseWithLocation {
	r.localPath = localPath
	return r
}

func (r releaseWithLocation) AsLocalRelease(localPath string) LocalRelease {
	return LocalRelease{
		ReleaseID: r.ReleaseID(),
		LocalPath: localPath,
	}
}

func (r releaseWithLocation) ReleaseSourceID() string {
	return r.remoteSourceID
}

func (r releaseWithLocation) AsRemoteRelease() RemoteRelease {
	return RemoteRelease{ReleaseID: r.ReleaseID(), RemotePath: r.RemotePath()}
}
