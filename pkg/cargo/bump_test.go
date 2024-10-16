package cargo

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-github/v50/github"

	. "github.com/onsi/gomega"

	fakes "github.com/pivotal-cf/kiln/internal/component/fakes_internal"
)

func TestCalculateBumps(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)

	t.Run("when the components stay the same", func(t *testing.T) {
		please.Expect(CalculateBumps([]BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
		}, []BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
		})).To(HaveLen(0))
	})

	t.Run("when a component is bumped", func(t *testing.T) {
		please.Expect(CalculateBumps([]BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
			{Name: "b", Version: "2"},
		}, []BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
			{Name: "b", Version: "1"},
		})).To(Equal([]Bump{
			{Name: "b", From: BOSHReleaseTarballLock{Name: "b", Version: "1"}, To: BOSHReleaseTarballLock{Name: "b", Version: "2"}},
		}),
			"it returns the changed lock",
		)
	})

	t.Run("when many but not all components are bumped", func(t *testing.T) {
		please.Expect(CalculateBumps([]BOSHReleaseTarballLock{
			{Name: "a", Version: "2"},
			{Name: "b", Version: "1"},
			{Name: "c", Version: "2"},
		}, []BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
			{Name: "b", Version: "1"},
			{Name: "c", Version: "1"},
		})).To(Equal([]Bump{
			{Name: "a", From: BOSHReleaseTarballLock{Name: "a", Version: "1"}, To: BOSHReleaseTarballLock{Name: "a", Version: "2"}},
			{Name: "c", From: BOSHReleaseTarballLock{Name: "c", Version: "1"}, To: BOSHReleaseTarballLock{Name: "c", Version: "2"}},
		}),
			"it returns all the bumps",
		)
	})

	t.Run("when a component is removed", func(t *testing.T) {
		please.Expect(CalculateBumps([]BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
		}, []BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
			{Name: "b", Version: "1"},
		})).To(HaveLen(0),
			"it does not return a bump",
		)
	})

	t.Run("when a component is added", func(t *testing.T) {
		// I'm not sure what we actually want to do here?
		// Is this actually a bump? Not really...

		please.Expect(CalculateBumps([]BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
			{Name: "b", Version: "1"},
		}, []BOSHReleaseTarballLock{
			{Name: "a", Version: "1"},
		})).To(Equal([]Bump{
			{Name: "b", From: BOSHReleaseTarballLock{}, To: BOSHReleaseTarballLock{Name: "b", Version: "1"}},
		}),
			"it returns the component as a bump",
		)
	})
}

func TestWinfsVersionBump(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)

	t.Run("when the winfs version is not bumped", func(t *testing.T) {
		please.Expect(WinfsVersionBump(false, "2.61.0", []Bump{
			{Name: "b", From: BOSHReleaseTarballLock{Version: "1"}, To: BOSHReleaseTarballLock{Version: "2"}},
		})).To(Equal([]Bump{
			{Name: "b", From: BOSHReleaseTarballLock{Version: "1"}, To: BOSHReleaseTarballLock{Version: "2"}},
		}))
	})

	t.Run("when the winfs version is bumped", func(t *testing.T) {
		please.Expect(WinfsVersionBump(true, "2.61.0", []Bump{
			{Name: "b", From: BOSHReleaseTarballLock{Version: "1"}, To: BOSHReleaseTarballLock{Version: "2"}},
		})).To(Equal([]Bump{
			{Name: "b", From: BOSHReleaseTarballLock{Version: "1"}, To: BOSHReleaseTarballLock{Version: "2"}},
			{Name: "windowsfs-release", From: BOSHReleaseTarballLock{Version: ""}, To: BOSHReleaseTarballLock{Version: "2.61.0"}},
		}))
	})
}

func TestInternal_addReleaseNotes(t *testing.T) {
	please := NewWithT(t)

	var ltsCallCount int

	releaseLister := new(fakes.RepositoryReleaseLister)
	releaseLister.ListReleasesStub = func(ctx context.Context, org string, repo string, options *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
		switch repo {
		case "lts-peach-release":
			switch ltsCallCount {
			case 0:
				ltsCallCount++
				return []*github.RepositoryRelease{
					{Body: strPtr("stored"), TagName: strPtr("1.1.0")},
					{Body: strPtr("served"), TagName: strPtr("2.0.1")},
					{Body: strPtr("plated"), TagName: strPtr("2.0.0")},
					{Body: strPtr("labeled"), TagName: strPtr("1.0.1")},
					{Body: strPtr("chopped"), TagName: strPtr("0.2.2")},
					{Body: strPtr("preserved"), TagName: strPtr("1.0.0")},
					{Body: strPtr("cleaned"), TagName: strPtr("0.2.1")},
					{Body: strPtr("ripe"), TagName: strPtr("0.1.3")},
					{Body: strPtr("unripe"), TagName: strPtr("0.1.2")},
					{Body: strPtr("flower"), TagName: strPtr("0.1.1")},
					{Body: strPtr("growing"), TagName: strPtr("0.1.0")},
				}, githubResponse(t, 200), nil
			default:
				ltsCallCount++
				return nil, nil, errors.New("ERROR")
			}
		}
		t.Errorf("unexpected repo: %q", repo)
		return nil, nil, nil
	}

	result, err := releaseNotes(
		context.Background(),
		Kilnfile{
			Releases: []BOSHReleaseTarballSpecification{
				{
					Name: "mango",
				},
				{
					Name:             "peach",
					GitHubRepository: "https://github.com/pivotal-cf/lts-peach-release",
				},
			},
		},
		BumpList{
			{
				Name: "peach",
				To:   BOSHReleaseTarballLock{Version: "2.0.1"}, // served
				From: BOSHReleaseTarballLock{Version: "0.1.3"}, // ripe
			},
			{
				Name: "mango",
				To:   BOSHReleaseTarballLock{Version: "10"},
				From: BOSHReleaseTarballLock{Version: "9"},
			},
		}, func(ctx context.Context, kilnfile Kilnfile, lock BOSHReleaseTarballLock) ([]repositoryReleaseLister, error) {
			var r []repositoryReleaseLister
			r = append(r, releaseLister)
			return r, nil
		})
	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(result).To(HaveLen(2))

	please.Expect(ltsCallCount).To(Equal(1))

	please.Expect(result[0].ReleaseNotes()).To(Equal("served\nplated\nstored\nlabeled\npreserved\nchopped\ncleaned"))
}

func Test_deduplicateReleasesWithTheSameTagName(t *testing.T) {
	please := NewWithT(t)
	b := Bump{
		Releases: []*github.RepositoryRelease{
			{TagName: ptr("Y")},
			{TagName: ptr("1")},
			{TagName: ptr("2")},
			{TagName: ptr("3")},
			{TagName: ptr("3")},
			{TagName: ptr("3")},
			{TagName: ptr("X")},
			{TagName: ptr("2")},
			{TagName: ptr("4")},
			{TagName: ptr("4")},
		},
	}
	b = deduplicateReleasesWithTheSameTagName(b)
	tags := make([]string, 0, len(b.Releases))
	for _, r := range b.Releases {
		tags = append(tags, r.GetTagName())
	}
	please.Expect(tags).To(Equal([]string{
		"Y",
		"1",
		"2",
		"3",
		"X",
		"4",
	}))
}

func githubResponse(t *testing.T, status int) *github.Response {
	t.Helper()

	return &github.Response{
		Response: &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
		},
	}
}

func strPtr(n string) *string { return &n }
