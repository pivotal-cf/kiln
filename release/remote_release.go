package release

type RemoteRelease interface {
	ReleaseID() ReleaseID
	RemotePath() string
	Satisfies(set ReleaseRequirement) bool
	StandardizedFilename() string
	WithLocalPath(string) ReleaseWithLocation
	AsLocalRelease(string) LocalRelease
}
