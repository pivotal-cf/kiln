package component_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/Masterminds/semver"

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
		releaseGetter := new(fakes.ReleaseByTagGetter)
		downloader := new(fakes.ReleaseAssetDownloader)

		releaseGetter.GetReleaseByTagReturns(
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
		downloader.DownloadReleaseAssetReturns(file, "", nil)

		ctx := context.TODO()

		lock, found, err := component.LockFromGithubRelease(ctx, downloader, "cloudfoundry", component.Spec{
			Name:    "routing",
			Version: "0.226.0",
			GitRepositories: []string{
				"https://github.com/cloudfoundry/routing-release",
			},
		}, component.GetGithubReleaseWithTag(releaseGetter, "0.226.0"))

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

			damnIt.Expect(downloader.DownloadReleaseAssetCallCount()).To(Ω.Equal(1))
			_, org, repo, build, client := downloader.DownloadReleaseAssetArgsForCall(0)
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

			damnIt.Expect(releaseGetter.GetReleaseByTagCallCount()).To(Ω.Equal(1))
			_, org, repo, tag := releaseGetter.GetReleaseByTagArgsForCall(0)
			damnIt.Expect(org).To(Ω.Equal("cloudfoundry"))
			damnIt.Expect(repo).To(Ω.Equal("routing-release"))
			damnIt.Expect(tag).To(Ω.Equal("0.226.0"))
		})
	})
}

func TestGetGithubReleaseWithTag(t *testing.T) {
	t.Run("when get release with tag api request fails", func(t *testing.T) {
		damnIt := Ω.NewWithT(t)

		releaseGetter := new(fakes.ReleaseByTagGetter)

		releaseGetter.GetReleaseByTagReturns(
			&github.RepositoryRelease{},
			&github.Response{Response: &http.Response{StatusCode: http.StatusUnauthorized}},
			errors.New("banana"),
		)

		ctx := context.TODO()

		fn := component.GetGithubReleaseWithTag(releaseGetter, "0.226.0")
		_, err := fn(ctx, "org", "repo")
		damnIt.Expect(err).To(Ω.HaveOccurred())
	})

	t.Run("when the status code is unauthorized and the error is nil", func(t *testing.T) {
		// yes this happened... how is this not an error?
		damnIt := Ω.NewWithT(t)

		defer func() {
			r := recover()
			if r != nil {
				t.Error("it should not panic")
			}
		}()

		releaseGetter := new(fakes.ReleaseByTagGetter)

		releaseGetter.GetReleaseByTagReturns(nil, &github.Response{
			Response: &http.Response{
				StatusCode: http.StatusUnauthorized,
			},
		}, nil)

		ctx := context.TODO()

		fn := component.GetGithubReleaseWithTag(releaseGetter, "0.226.0")
		_, err := fn(ctx, "org", "repo")
		damnIt.Expect(err).To(Ω.HaveOccurred())
	})
}

func TestGetReleaseMatchingConstraint(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("when get release with tag api request fails", func(t *testing.T) {
		damnIt := Ω.NewWithT(t)

		releaseGetter := new(fakes.ReleasesLister)

		releaseGetter.ListReleasesReturnsOnCall(0,
			[]*github.RepositoryRelease{
				{TagName: strPtr("3.0.0")},
				{TagName: strPtr("2.2.1")},
				{TagName: strPtr("2.2.0")},
				{TagName: strPtr("2.1.0")},
				{TagName: strPtr("2.0.4")},
				{TagName: strPtr("2.0.3")},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)
		releaseGetter.ListReleasesReturnsOnCall(1,
			[]*github.RepositoryRelease{
				{TagName: strPtr("2.0.0-beta.1")},
				{TagName: strPtr("1.9.42")},
				{TagName: strPtr("1.8.0")},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)
		releaseGetter.ListReleasesReturnsOnCall(2,
			[]*github.RepositoryRelease{
				{TagName: strPtr("2.0.0-alpha.0")},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)
		releaseGetter.ListReleasesReturnsOnCall(3,
			[]*github.RepositoryRelease{
				{TagName: strPtr("1.7.5")},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)

		ctx := context.TODO()

		c, err := semver.NewConstraint("~2.0")
		damnIt.Expect(err).NotTo(Ω.HaveOccurred())
		fn := component.GetReleaseMatchingConstraint(releaseGetter, c)
		rel, err := fn(ctx, "org", "repo")
		damnIt.Expect(err).NotTo(Ω.HaveOccurred())
		damnIt.Expect(rel.GetTagName()).To(Ω.Equal("2.0.4"))
		damnIt.Expect(releaseGetter.ListReleasesCallCount()).To(Ω.Equal(3))
	})
}

func TestGetOwnerAndRepo(t *testing.T) {
	tests := []struct {
		name        string
		givenURLStr string
		wantOwner   string
		wantRepo    string
	}{
		{name: "https", givenURLStr: "https://github.com/some-org/some-repo.git", wantOwner: "some-org", wantRepo: "some-repo"},
		{name: "git", givenURLStr: "git@github.com:some-org/some-repo.git", wantOwner: "some-org", wantRepo: "some-repo"},

		{name: "empty", givenURLStr: "", wantOwner: "", wantRepo: ""},
		{name: "incomplete", givenURLStr: "github.com/example", wantOwner: "", wantRepo: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwner, gotRepo := component.OwnerAndRepoFromGitHubURI(tt.givenURLStr)
			if gotOwner != tt.wantOwner {
				t.Errorf("OwnerAndRepoFromGitHubURI() gotOwner = %v, want %v", gotOwner, tt.wantOwner)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("OwnerAndRepoFromGitHubURI() gotRepo = %v, want %v", gotRepo, tt.wantRepo)
			}
		})
	}
}

/*
func TestDownloadReleaseAsset(t *testing.T) {
	//Mocking up the Lock we'll need to test
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int64) *int64 { return &i }
	releaseGetter := new(fakes.ReleaseByTagGetter)
	downloader := new(fakes.ReleaseAssetDownloader)

	releaseGetter.GetReleaseByTagReturns(
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
	downloader.DownloadReleaseAssetReturns(file, "", nil)

	ctx := context.TODO()

	testReleaseLock, _, _ := component.LockFromGithubRelease(ctx, downloader, "cloudfoundry", component.Spec{
		Name:    "routing",
		Version: "0.226.0",
		GitRepositories: []string{
			"https://github.com/cloudfoundry/routing-release",
		},
	}, component.GetGithubReleaseWithTag(releaseGetter, "0.226.0"))

}
*/
