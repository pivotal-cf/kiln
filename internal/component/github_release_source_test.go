package component_test

import (
	"context"
	"errors"
	"net/http"
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
	release, response, err := grs.Client.Repositories.GetReleaseByTag(context.TODO(), "cloudfoundry", "routing-release", "0.226.0")
	if err != nil {
		t.Error(err)
	}
	// t.Log(request)
	if release.Assets != nil {
		for _, a := range release.Assets {
			t.Log(a)
		}
	}

	for key, val := range response.Header {
		t.Logf("%s: %s", key, val)
	}
}

func TestGithubReleaseSource_ComponentLockFromGithubRelease(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("when release is found in first repo", func(t *testing.T) {
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

		t.Run("it returns success stuff", func(t *testing.T) {
			damnIt := Ω.NewWithT(t)

			damnIt.Expect(err).NotTo(Ω.HaveOccurred())
			damnIt.Expect(found).To(Ω.BeTrue())
		})

		t.Run("it sets the lock fields properly", func(t *testing.T) {
			damnIt := Ω.NewWithT(t)

			damnIt.Expect(lock.Version).To(Ω.Equal("0.226.0"))
			damnIt.Expect(lock.Name).To(Ω.Equal("routing"))
			damnIt.Expect(lock.Version).To(Ω.Equal("0.226.0"))
			damnIt.Expect(lock.RemoteSource).To(Ω.Equal(component.ReleaseSourceTypeGithub))
			damnIt.Expect(lock.RemotePath).To(Ω.Equal("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz"))
		})

		t.Run("it makes the right request", func(t *testing.T) {
			damnIt := Ω.NewWithT(t)

			damnIt.Expect(tagger.GetReleaseByTagCallCount()).To(Ω.Equal(1))
			_, org, repo, tag := tagger.GetReleaseByTagArgsForCall(0)
			damnIt.Expect(org).To(Ω.Equal("cloudfoundry"))
			damnIt.Expect(repo).To(Ω.Equal("routing-release"))
			damnIt.Expect(tag).To(Ω.Equal("0.226.0"))

			t.Run("it sets the tarball hash", func(t *testing.T) {
				t.Skip()
				doubleDamnIt := Ω.NewWithT(t)
				doubleDamnIt.Expect(lock.SHA1).To(Ω.Equal("???"))
			})
		})
	})

	t.Run("the github api request fails", func(t *testing.T) {
		damnIt := Ω.NewWithT(t)

		tagger := new(fakes.GetReleaseByTagger)

		tagger.GetReleaseByTagReturns(&github.RepositoryRelease{}, nil, errors.New("banana"))

		ctx := context.TODO()

		_, _, err := component.LockFromGithubRelease(ctx, tagger, "cloudfoundry", component.Spec{
			Name:    "routing",
			Version: "0.226.0",
			GitRepositories: []string{
				"https://github.com/cloudfoundry/routing-release",
			},
		})
		damnIt.Expect(err).To(Ω.HaveOccurred())
	})

	t.Run("the status code is unauthorized and the error is nil", func(t *testing.T) {
		// yes this happened... how is this not an error
		damnIt := Ω.NewWithT(t)

		defer func() {
			r := recover()
			if r != nil {
				t.Error("it should not panic")
			}
		}()

		tagger := new(fakes.GetReleaseByTagger)

		tagger.GetReleaseByTagReturns(nil, &github.Response{
			Response: &http.Response{
				StatusCode: http.StatusUnauthorized,
			},
		}, nil)

		ctx := context.TODO()

		_, _, err := component.LockFromGithubRelease(ctx, tagger, "cloudfoundry", component.Spec{
			Name:    "routing",
			Version: "0.226.0",
			GitRepositories: []string{
				"https://github.com/cloudfoundry/routing-release",
			},
		})
		damnIt.Expect(err).To(Ω.HaveOccurred())
	})
}
