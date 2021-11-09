package component_test

import (
	"context"
	"os"
	"testing"

	Ω "github.com/onsi/gomega"

	"github.com/google/go-github/v40/github"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestListAllOfTheCrap(t *testing.T) {

	grs := component.NewGithubReleaseSource(cargo.ReleaseSourceConfig{
		Type:        component.ReleaseSourceTypeGithub,
		GithubToken: os.Getenv("GITHUB_TOKEN"),
	})
	//grs.ListAllOfTheCrap(context.TODO(), "cloudfoundry")

	//grs.Client.Repositories.GetReleaseByTag()
	release, _, _ := grs.Client.Repositories.GetReleaseByTag(context.TODO(), "cloudfoundry", "routing-release", "0.226.0")
	for _, a := range release.Assets {
		t.Log(a)
	}

}

func TestGithubReleaseSource_ComponentLockFromGithubRelease(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("release is found in first repo", func(t *testing.T) {
		damnit := Ω.NewWithT(t)

		tagger := new(fakes.GetReleaseByTagger)

		tagger.GetReleaseByTagReturns(&github.RepositoryRelease{
			TagName: strPtr("0.226.0"),
			Assets: []*github.ReleaseAsset{
				{
					Name:               strPtr("routing-0.226.0.tgz.sha256"),
					BrowserDownloadURL: strPtr("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz.sha256"),
				},
				{
					Name:               strPtr("routing-0.226.0.tgz"),
					BrowserDownloadURL: strPtr("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz"),
				},
			},
		}, nil, nil)

		ctx := context.TODO()

		lock, found, err := component.LockFromGithubRelease(ctx, tagger, "cloudfoundry", component.Spec{
			Name:    "routing",
			Version: "0.226.0",
			GitRepositories: []string{
				"https://github.com/cloudfoundry/routing-release",
			},
		})

		damnit.Expect(err).NotTo(Ω.HaveOccurred())
		damnit.Expect(found).To(Ω.BeTrue())
		damnit.Expect(lock.Version).To(Ω.Equal("0.226.0"))

		damnit.Expect(tagger.GetReleaseByTagCallCount()).To(Ω.Equal(1))
		_, org, repo, tag := tagger.GetReleaseByTagArgsForCall(0)
		damnit.Expect(org).To(Ω.Equal("cloudfoundry"))
		damnit.Expect(repo).To(Ω.Equal("routing-release"))
		damnit.Expect(tag).To(Ω.Equal("0.226.0"))

		damnit.Expect(lock.Name).To(Ω.Equal("routing"))
		damnit.Expect(lock.Version).To(Ω.Equal("0.226.0"))
		damnit.Expect(lock.RemoteSource).To(Ω.Equal(component.ReleaseSourceTypeGithub))
		damnit.Expect(lock.RemotePath).To(Ω.Equal("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz"))
		// damnit.Expect(lock.SHA1).To(Ω.Equal("???")) // not sure how we will get this... maybe we should just switch to using sha256 everywhere?
	})
}
