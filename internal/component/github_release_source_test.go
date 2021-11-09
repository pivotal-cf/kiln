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

	t.Run("release is found in first repo", func(t *testing.T) {
		tagger := new(fakes.GetReleaseByTagger)

		tagger.GetReleaseByTagReturns(&github.RepositoryRelease{
			TagName: strPtr("0.226.0"),
			URL: strPtr("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz"),
		}, nil, nil)

		ctx := context.TODO()

		lock, _, _ := component.LockFromGithubRelease(ctx, tagger, "cloudfoundry", component.Spec{
			Name:    "routing",
			Version: "0.226.0",
			Repositories: []string{
				"https://github.com/cloudfoundry/routing-release",
			},
		})

		damnit.Expect(lock.Version).To(Ω.Equal("0.226.0"))

		damnit.Expect(tagger.GetReleaseByTagCallCount()).To(Ω.Equal(1))
		_, org, repo, tag := tagger.GetReleaseByTagArgsForCall(0)
		damnit.Expect(org).To(Ω.Equal("cloudfoundry"))
		damnit.Expect(repo).To(Ω.Equal("routing-release"))
		damnit.Expect(tag).To(Ω.Equal("0.226.0"))

		damnit.Expect(lock.Name).To(Ω.Equal("routing"))
		damnit.Expect(lock.Version).To(Ω.Equal("0.226.0"))
		damnit.Expect(lock.RemoteSource).To(Ω.Equal(component.ReleaseSourceTypeGithub))
		damnit.Expect(lock.RemoteSource).To(Ω.Equal("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz"))
		// damnit.Expect(lock.SHA1).To(Ω.Equal("???")) // not sure how we will get this... maybe we should just switch to using sha256 everywhere?
	})
}
