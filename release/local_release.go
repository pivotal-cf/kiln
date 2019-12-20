package release

//go:generate counterfeiter -o ./fakes/local_release.go --fake-name LocalRelease . LocalRelease
type LocalRelease interface {
	LocalPath() string
	Satisfies(set ReleaseRequirement) bool
}

