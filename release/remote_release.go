package release

type RemoteRelease interface {
	RemotePath() string
	ReleaseID() ReleaseID
	AsLocal(string) LocalRelease
	StandardizedFilename() string
}

