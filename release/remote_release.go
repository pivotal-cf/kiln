package release

type RemoteRelease interface {
	AsLocal(string) LocalRelease
	ReleaseID() ReleaseID
	RemotePath() string
	Satisfies(set ReleaseRequirement) bool
	StandardizedFilename() string
	WithLocalPath(string) ReleaseWithLocation
}

