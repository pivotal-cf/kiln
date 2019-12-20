package release

//go:generate counterfeiter -o ./fakes/local_release.go --fake-name LocalRelease . LocalRelease
type LocalRelease interface {
	Satisfies(set ReleaseRequirement) bool
	LocalPath() string
}

