package component_test

import (
	"bytes"
	"context"
	"errors"
	"io"
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
	t.SkipNow()

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

type SetTrueOnClose struct {
	io.Reader
	CloseCalled bool
}

func (c *SetTrueOnClose) Close() error {
	c.CloseCalled = true
	return nil
}

func TestGithubReleaseSource_ComponentLockFromGithubRelease(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(n int64) *int64 { return &n }

	t.Run("when release is found in first repo", func(t *testing.T) {
		ghRepoAPI := new(fakes.GitHubRepositoryAPI)

		ghRepoAPI.GetReleaseByTagReturns(
			&github.RepositoryRelease{
				TagName: strPtr("0.226.0"),
				Assets: []*github.ReleaseAsset{
					{
						Name:               strPtr("routing-0.226.0.tgz.sha256"),
						BrowserDownloadURL: strPtr("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz.sha256"),
					},
					{
						Name:               strPtr("routing-0.226.0.tgz"),
						BrowserDownloadURL: strPtr("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz"),
						ID:                 intPtr(420),
					},
				},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)

		file := &SetTrueOnClose{Reader: bytes.NewBufferString("hello")}
		ghRepoAPI.DownloadReleaseAssetReturns(file, "", nil)

		ctx := context.TODO()

		lock, found, err := component.LockFromGithubRelease(ctx, ghRepoAPI, "cloudfoundry", component.Spec{
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

		t.Run("it downloads the file", func(t *testing.T) {
			damnIt := Ω.NewWithT(t)

			damnIt.Expect(ghRepoAPI.DownloadReleaseAssetCallCount()).To(Ω.Equal(1))
			_, org, repo, build, client := ghRepoAPI.DownloadReleaseAssetArgsForCall(0)
			damnIt.Expect(org).To(Ω.Equal("cloudfoundry"))
			damnIt.Expect(repo).To(Ω.Equal("routing-release"))
			damnIt.Expect(build).To(Ω.Equal(int64(420)))
			damnIt.Expect(client).NotTo(Ω.BeNil())

			t.Run("it sets the tarball hash", func(t *testing.T) {
				doubleDamnIt := Ω.NewWithT(t)
				doubleDamnIt.Expect(lock.SHA1).To(Ω.Equal("aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"))
				doubleDamnIt.Expect(file.CloseCalled).To(Ω.BeTrue())
			})
		})

		t.Run("it makes the right request", func(t *testing.T) {
			damnIt := Ω.NewWithT(t)

			damnIt.Expect(ghRepoAPI.GetReleaseByTagCallCount()).To(Ω.Equal(1))
			_, org, repo, tag := ghRepoAPI.GetReleaseByTagArgsForCall(0)
			damnIt.Expect(org).To(Ω.Equal("cloudfoundry"))
			damnIt.Expect(repo).To(Ω.Equal("routing-release"))
			damnIt.Expect(tag).To(Ω.Equal("0.226.0"))
		})
	})

	t.Run("the github api request fails", func(t *testing.T) {
		damnIt := Ω.NewWithT(t)

		ghRepoAPI := new(fakes.GitHubRepositoryAPI)

		ghRepoAPI.GetReleaseByTagReturns(
			&github.RepositoryRelease{},
			&github.Response{Response: &http.Response{StatusCode: http.StatusUnauthorized}},
			errors.New("banana"),
		)

		ctx := context.TODO()

		_, _, err := component.LockFromGithubRelease(ctx, ghRepoAPI, "cloudfoundry", component.Spec{
			Name:    "routing",
			Version: "0.226.0",
			GitRepositories: []string{
				"https://github.com/cloudfoundry/routing-release",
			},
		})
		damnIt.Expect(err).To(Ω.HaveOccurred())
	})

	t.Run("the status code is unauthorized and the error is nil", func(t *testing.T) {
		// yes this happened... how is this not an error?
		damnIt := Ω.NewWithT(t)

		defer func() {
			r := recover()
			if r != nil {
				t.Error("it should not panic")
			}
		}()

		ghRepoAPI := new(fakes.GitHubRepositoryAPI)

		ghRepoAPI.GetReleaseByTagReturns(nil, &github.Response{
			Response: &http.Response{
				StatusCode: http.StatusUnauthorized,
			},
		}, nil)

		ctx := context.TODO()

		_, _, err := component.LockFromGithubRelease(ctx, ghRepoAPI, "cloudfoundry", component.Spec{
			Name:    "routing",
			Version: "0.226.0",
			GitRepositories: []string{
				"https://github.com/cloudfoundry/routing-release",
			},
		})
		damnIt.Expect(err).To(Ω.HaveOccurred())
	})
}
