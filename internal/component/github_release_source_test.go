package component_test

import (
	"context"
	"testing"

	Ω "github.com/onsi/gomega"

	"github.com/google/go-github/v40/github"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
)

func TestListAllOfTheCrap(t *testing.T) {
	t.SkipNow()

	//grs := component.NewGithubReleaseSource(cargo.ReleaseSourceConfig{
	//	Type: component.ReleaseSourceTypeGithub,
	//	GithubToken: os.Getenv("GITHUB_TOKEN"),
	//})
	// grs.ListAllOfTheCrap(context.TODO(), "cloudfoundry")

	// grs.Client.Repositories.GetReleaseByTag()
}

func TestGithubReleaseSource_ComponentLockFromGithubRelease(t *testing.T) {
	damnit := Ω.NewWithT(t)
	strPtr := func(s string) *string { return &s }

	t.Run("release is found", func(t *testing.T) {
		tagger := new(fakes.GetReleaseByTagger)

		tagger.GetReleaseByTagReturns(&github.RepositoryRelease{
			TagName: strPtr("0.226.0"),
		}, nil, nil)

		ctx := context.TODO()

		lock, _, _ := component.LockFromGithubRelease(ctx, tagger, component.Spec{
			Name:    "routing",
			Version: "0.226.0",
		})

		damnit.Expect(lock.Version).To(Ω.Equal("0.226.0"))
		// damnit.Expect(lock.Name).To(Ω.Equal("routing"))
	})
}
