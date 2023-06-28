package component_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/google/go-github/v40/github"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestListAllOfTheCrap(t *testing.T) {
	t.SkipNow()

	grs := component.NewGithubReleaseSource(cargo.ReleaseSourceConfig{
		Type:        component.ReleaseSourceTypeGithub,
		GithubToken: os.Getenv("GITHUB_TOKEN"),
		Org:         "cloudfoundry",
	})
	// grs.ListAllOfTheCrap(context.TODO(), "cloudfoundry")

	// grs.Client.Repositories.GetReleaseByTag()
	release, response, err := grs.GetReleaseByTag(context.TODO(), "cloudfoundry", "routing-release", "0.226.0")
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

		const owner = "cloudfoundry"

		releaseGetter.GetReleaseByTagReturns(
			&github.RepositoryRelease{
				TagName: strPtr("v0.226.0"),
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

		output := bytes.NewBuffer(nil)
		grsMock := &component.GithubReleaseSource{
			Logger:                 log.New(output, "[Github release source] ", log.Default().Flags()),
			ReleaseAssetDownloader: downloader,
			ReleaseByTagGetter:     releaseGetter,
			ReleaseSourceConfig: cargo.ReleaseSourceConfig{
				Org: owner,
			},
		}

		lock, err := grsMock.GetMatchedRelease(cargo.ComponentSpec{
			Name:             "routing",
			Version:          "0.226.0",
			GitHubRepository: "https://github.com/cloudfoundry/routing-release",
		})

		t.Run("it returns success stuff", func(t *testing.T) {
			damnIt := NewWithT(t)

			damnIt.Expect(err).NotTo(HaveOccurred())
		})

		t.Run("it sets the lock fields properly", func(t *testing.T) {
			damnIt := NewWithT(t)

			damnIt.Expect(lock.Name).To(Equal("routing"))
			damnIt.Expect(lock.Version).To(Equal("0.226.0"))
			damnIt.Expect(lock.RemoteSource).To(Equal(owner))
			damnIt.Expect(lock.RemotePath).To(Equal("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz"))
		})

		t.Run("it downloads the file", func(t *testing.T) {
			damnIt := NewWithT(t)

			damnIt.Expect(downloader.DownloadReleaseAssetCallCount()).To(Equal(1))
			_, org, repo, build, client := downloader.DownloadReleaseAssetArgsForCall(0)
			damnIt.Expect(org).To(Equal("cloudfoundry"))
			damnIt.Expect(repo).To(Equal("routing-release"))
			damnIt.Expect(build).To(Equal(int64(420)))
			damnIt.Expect(client).NotTo(BeNil())

			t.Run("it sets the tarball hash", func(t *testing.T) {
				doubleDamnIt := NewWithT(t)
				doubleDamnIt.Expect(lock.SHA1).To(Equal("aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"))
				doubleDamnIt.Expect(file.CloseCalled).To(BeTrue())
			})
		})

		t.Run("it makes the right request", func(t *testing.T) {
			damnIt := NewWithT(t)

			damnIt.Expect(releaseGetter.GetReleaseByTagCallCount()).To(Equal(1))
			_, org, repo, tag := releaseGetter.GetReleaseByTagArgsForCall(0)
			damnIt.Expect(org).To(Equal("cloudfoundry"))
			damnIt.Expect(repo).To(Equal("routing-release"))
			damnIt.Expect(tag).To(Equal("v0.226.0"))
		})
	})

	t.Run("when the github release tag name has a v prefix", func(t *testing.T) {
		// Given...
		releaseGetter := new(fakes.ReleaseByTagGetter)
		downloader := new(fakes.ReleaseAssetDownloader)

		const owner = "cloudfoundry"

		releaseGetter.GetReleaseByTagReturnsOnCall(0,
			nil,
			&github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
			nil,
		)
		releaseGetter.GetReleaseByTagReturns(
			&github.RepositoryRelease{
				TagName: strPtr("v0.226.0"),
				Assets: []*github.ReleaseAsset{
					{
						Name:               strPtr("routing-0.226.0.tgz"),
						BrowserDownloadURL: strPtr("https://github.com/cloudfoundry/routing-release/releases/download/v0.226.0/routing-0.226.0.tgz"),
						ID:                 intPtr(420),
					},
				},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)

		file := &SetTrueOnClose{Reader: bytes.NewBufferString("hello")}
		downloader.DownloadReleaseAssetReturns(file, "", nil)

		output := bytes.NewBuffer(nil)
		grsMock := &component.GithubReleaseSource{
			Logger:                 log.New(output, "[Github release source] ", log.Default().Flags()),
			ReleaseAssetDownloader: downloader,
			ReleaseByTagGetter:     releaseGetter,
			ReleaseSourceConfig: cargo.ReleaseSourceConfig{
				Org: owner,
			},
		}

		// When...
		lock, err := grsMock.GetMatchedRelease(cargo.ComponentSpec{
			Name:             "routing",
			Version:          "0.226.0",
			GitHubRepository: "https://github.com/cloudfoundry/routing-release",
		})

		// Then...
		t.Run("it returns success stuff", func(t *testing.T) {
			damnIt := NewWithT(t)

			damnIt.Expect(err).NotTo(HaveOccurred())
		})

		t.Run("it sets the lock fields properly", func(t *testing.T) {
			damnIt := NewWithT(t)

			damnIt.Expect(lock.Name).To(Equal("routing"))
			damnIt.Expect(lock.Version).To(Equal("0.226.0"))
			damnIt.Expect(lock.RemoteSource).To(Equal(owner))
			damnIt.Expect(lock.RemotePath).To(Equal("https://github.com/cloudfoundry/routing-release/releases/download/v0.226.0/routing-0.226.0.tgz"))
		})

		t.Run("it makes the right request", func(t *testing.T) {
			damnIt := NewWithT(t)

			damnIt.Expect(releaseGetter.GetReleaseByTagCallCount()).To(Equal(2))

			_, _, _, tag := releaseGetter.GetReleaseByTagArgsForCall(0)
			damnIt.Expect(tag).To(Equal("v0.226.0"))

			_, _, _, tag = releaseGetter.GetReleaseByTagArgsForCall(1)
			damnIt.Expect(tag).To(Equal("0.226.0"))
		})
	})
}

func TestGithubReleaseSource_FindReleaseVersion(t *testing.T) {
	t.Run("when spec contains a version string other than semver", func(t *testing.T) {
		s := cargo.ComponentSpec{
			Version: "garbage",
		}
		grs := component.NewGithubReleaseSource(cargo.ReleaseSourceConfig{Type: component.ReleaseSourceTypeGithub, GithubToken: "fake_token", Org: "cloudfoundry"})
		_, err := grs.FindReleaseVersion(s, false)

		t.Run("it returns an error about version not being specific", func(t *testing.T) {
			damnIt := NewWithT(t)
			damnIt.Expect(err).To(HaveOccurred())
			damnIt.Expect(err.Error()).To(ContainSubstring("expected version to be a constraint"))
		})
	})

	t.Run("noDownload is true", func(t *testing.T) {
		please := NewWithT(t)

		downloader := new(fakes.ReleaseAssetDownloader)
		downloader.DownloadReleaseAssetReturns(nil, "", fmt.Errorf("this is a mistake! I'm not supposed to be here!"))

		releaseName := "bpm"
		version := "v1.1.19"
		assetName := "bpm-1.1.19.tgz"
		assetID := int64(123)
		lister := new(fakes.ReleasesLister)
		listResponse := []*github.RepositoryRelease{{
			Name:    &releaseName,
			TagName: &version,
			Assets: []*github.ReleaseAsset{{
				Name: &assetName,
				ID:   &assetID,
			}},
		}}
		httpResponse := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
		returnCount := 0
		lister.ListReleasesStub = func(ctx context.Context, s1, s2 string, lo *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
			if returnCount != 0 {
				return nil, httpResponse, nil
			}
			returnCount++
			return listResponse, httpResponse, nil
		}

		s := cargo.ComponentSpec{
			Name:             releaseName,
			GitHubRepository: "https://github.com/cloudfoundry/bpm-release",
		}

		output := bytes.NewBuffer(nil)
		defer t.Log(output.String())
		grsMock := &component.GithubReleaseSource{
			Logger:                 log.New(output, "[test] ", log.Default().Flags()),
			ReleaseAssetDownloader: downloader,
			ReleasesLister:         lister,
			ReleaseSourceConfig: cargo.ReleaseSourceConfig{
				Type:        component.ReleaseSourceTypeGithub,
				Org:         "cloudfoundry",
				GithubToken: "fake-token",
			},
		}

		lock, err := grsMock.FindReleaseVersion(s, true)
		please.Expect(err).ToNot(HaveOccurred())

		please.Expect(lock.SHA1).To(Equal("not-calculated"))
		please.Expect(downloader.Invocations()).To(BeEmpty())
	})
}

func TestGithubReleaseSource_GetMatchedRelease(t *testing.T) {
	t.Run("when spec contains a version string other than semver", func(t *testing.T) {
		s := cargo.ComponentSpec{
			Version: ">1.0.0",
		}
		grs := component.NewGithubReleaseSource(cargo.ReleaseSourceConfig{Type: component.ReleaseSourceTypeGithub, GithubToken: "fake_token", Org: "cloudfoundry"})
		_, err := grs.GetMatchedRelease(s)

		t.Run("it returns an error about version not being specific", func(t *testing.T) {
			damnIt := NewWithT(t)
			damnIt.Expect(err).To(HaveOccurred())
			damnIt.Expect(err.Error()).To(ContainSubstring("expected version to be an exact version"))
		})
	})
}

func TestGetGithubReleaseWithTag(t *testing.T) {
	t.Run("when get release with tag api request fails", func(t *testing.T) {
		damnIt := NewWithT(t)

		releaseGetter := new(fakes.ReleaseByTagGetter)

		releaseGetter.GetReleaseByTagReturns(
			&github.RepositoryRelease{},
			&github.Response{Response: &http.Response{StatusCode: http.StatusUnauthorized}},
			errors.New("banana"),
		)

		ctx := context.TODO()

		output := bytes.NewBuffer(nil)
		grsMock := &component.GithubReleaseSource{
			Logger:             log.New(output, "[test] ", log.Default().Flags()),
			ReleaseByTagGetter: releaseGetter,
			ReleaseSourceConfig: cargo.ReleaseSourceConfig{
				Type:        component.ReleaseSourceTypeGithub,
				Org:         "cloudfoundry",
				GithubToken: "fake-token",
			},
		}
		s := cargo.ComponentSpec{
			Name:             "routing",
			Version:          "0.226.0",
			GitHubRepository: "https://github.com/cloudfoundry/routing-release",
		}

		_, err := grsMock.GetGithubReleaseWithTag(ctx, s)
		damnIt.Expect(err).To(HaveOccurred())
	})

	t.Run("when the status code is unauthorized and the error is nil", func(t *testing.T) {
		// yes this happened... how is this not an error?
		damnIt := NewWithT(t)

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

		output := bytes.NewBuffer(nil)
		grsMock := &component.GithubReleaseSource{
			Logger:             log.New(output, "[test] ", log.Default().Flags()),
			ReleaseByTagGetter: releaseGetter,
			ReleaseSourceConfig: cargo.ReleaseSourceConfig{
				Type:        component.ReleaseSourceTypeGithub,
				Org:         "cloudfoundry",
				GithubToken: "fake-token",
			},
		}
		s := cargo.ComponentSpec{
			Name:             "routing",
			Version:          "0.226.0",
			GitHubRepository: "https://github.com/cloudfoundry/routing-release",
		}

		_, err := grsMock.GetGithubReleaseWithTag(ctx, s)
		damnIt.Expect(err).To(HaveOccurred())
	})
}

func TestGetLatestMatchingRelease(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("when get release with tag api request fails", func(t *testing.T) {
		damnIt := NewWithT(t)

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

		output := bytes.NewBuffer(nil)
		grsMock := &component.GithubReleaseSource{
			Logger:         log.New(output, "[test] ", log.Default().Flags()),
			ReleasesLister: releaseGetter,
			ReleaseSourceConfig: cargo.ReleaseSourceConfig{
				Org: "test-org",
			},
		}

		s := cargo.ComponentSpec{
			Name:             "test",
			Version:          "~2.0",
			GitHubRepository: "git@github.com:test-org/test.git",
		}

		rel, err := grsMock.GetLatestMatchingRelease(context.TODO(), s)
		damnIt.Expect(err).NotTo(HaveOccurred())
		damnIt.Expect(rel.GetTagName()).To(Equal("2.0.4"))
		damnIt.Expect(releaseGetter.ListReleasesCallCount()).To(Equal(3))
	})

	t.Run("when some of the github releases tags have a v prefix", func(t *testing.T) {
		damnIt := NewWithT(t)

		releaseGetter := new(fakes.ReleasesLister)

		releaseGetter.ListReleasesReturnsOnCall(0,
			[]*github.RepositoryRelease{
				{TagName: strPtr("v2.1.0")},
				{TagName: strPtr("v2.0.4")},
				{TagName: strPtr("2.0.3")},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)
		releaseGetter.ListReleasesReturnsOnCall(1,
			[]*github.RepositoryRelease{
				{TagName: strPtr("1.8.0")},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)
		releaseGetter.ListReleasesReturnsOnCall(2,
			[]*github.RepositoryRelease{
				{TagName: strPtr("1.7.5")},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)

		output := bytes.NewBuffer(nil)
		grsMock := &component.GithubReleaseSource{
			Logger:         log.New(output, "[test] ", log.Default().Flags()),
			ReleasesLister: releaseGetter,
			ReleaseSourceConfig: cargo.ReleaseSourceConfig{
				Org: "test-org",
			},
		}

		s := cargo.ComponentSpec{
			Name:             "test",
			Version:          "~2.0",
			GitHubRepository: "git@github.com:test-org/test.git",
		}

		rel, err := grsMock.GetLatestMatchingRelease(context.TODO(), s)
		damnIt.Expect(err).NotTo(HaveOccurred())
		damnIt.Expect(rel.GetTagName()).To(Equal("v2.0.4"))
		damnIt.Expect(releaseGetter.ListReleasesCallCount()).To(Equal(3))
	})

	t.Run("component repo does not match release source org", func(t *testing.T) {
		// given
		var (
			githubOrg      = "banana"
			otherGitHubOrg = "orange"

			ctx  = context.Background()
			spec = cargo.ComponentSpec{
				GitHubRepository: "https://github.com/" + otherGitHubOrg + "/muffin",
			}
		)

		output := bytes.NewBuffer(nil)
		grsMock := &component.GithubReleaseSource{
			Logger: log.New(output, "[test] ", log.Default().Flags()),
			ReleaseSourceConfig: cargo.ReleaseSourceConfig{
				Org: githubOrg,
			},
		}

		// when
		_, err := grsMock.GetLatestMatchingRelease(ctx, spec)

		// then
		please := NewWithT(t)
		please.Expect(component.IsErrNotFound(err)).To(BeTrue())
	})
}

func TestDownloadReleaseAsset(t *testing.T) {
	t.SkipNow()

	grs := component.NewGithubReleaseSource(cargo.ReleaseSourceConfig{
		Type:        component.ReleaseSourceTypeGithub,
		GithubToken: os.Getenv("GITHUB_TOKEN"),
		Org:         "cloudfoundry",
	})
	testLock, err := grs.GetMatchedRelease(cargo.ComponentSpec{Name: "routing", Version: "0.226.0", GitHubRepository: "https://github.com/cloudfoundry/routing-release"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("when the release is downloaded", func(t *testing.T) {
		damnIt := NewWithT(t)
		tempDir := t.TempDir()
		t.Cleanup(func() {
			_ = os.RemoveAll(tempDir)
		})

		local, err := grs.DownloadRelease(tempDir, testLock)
		damnIt.Expect(err).NotTo(HaveOccurred())

		damnIt.Expect(local.LocalPath).NotTo(BeAnExistingFile(), "it creates the expected asset")
	})
}
