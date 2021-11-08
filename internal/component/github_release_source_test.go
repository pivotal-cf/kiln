package component_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-github/v40/github"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestListAllOfTheCrap(t *testing.T) {
	grs := component.NewGithubReleaseSource(cargo.ReleaseSourceConfig{Type: component.ReleaseSourceTypeGithub, GithubToken: os.Getenv("GITHUB_TOKEN")})
	grs.ListAllOfTheCrap(context.TODO(), "cloudfoundry")
}

func TestGithubReleaseSource_GetMatchedRelease(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	lister := new(fakes.GithubReleaseLister)
	lister.ListReleasesReturns([]*github.RepositoryRelease{
		{
			TagName: strPtr("0.226.0"),
		},
	}, nil, nil)
	grs := component.GithubReleaseSource{}
	_, found, err := grs.GetMatchedReleaseImpl(lister, component.Requirement{Version: "0.226.0"})
	if !found {
		panic("Error TestGithubReleaseSource_GetMatchedRelease: " + err.Error())
	}
	//Returns the Lock, but not sure what we want to do with it to determine success/failure
}
