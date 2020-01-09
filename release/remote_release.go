package release

type DeprecatedRemoteRelease interface {
	ReleaseID() ReleaseID
	RemotePath() string
	Satisfies(set ReleaseRequirement) bool
	StandardizedFilename() string
	WithLocalPath(string) ReleaseWithLocation
	AsLocalRelease(string) LocalRelease
	AsRemoteRelease() RemoteRelease
}

type RemoteRelease struct {
	ReleaseID  ReleaseID
	RemotePath string
}
