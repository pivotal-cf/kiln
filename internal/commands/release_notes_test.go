package commands

import (
	"bytes"
	"context"
	"io/ioutil"
	"reflect"
	"regexp"
	"testing"

	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v40/github"

	fakes "github.com/pivotal-cf/kiln/internal/commands/fakes_internal"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ jhanda.Command = ReleaseNotes{}

func TestReleaseNotes_Usage(t *testing.T) {
	please := Ω.NewWithT(t)

	rn := ReleaseNotes{}

	please.Expect(rn.Usage().Description).NotTo(Ω.BeEmpty())
	please.Expect(rn.Usage().ShortDescription).NotTo(Ω.BeEmpty())
	please.Expect(rn.Usage().Flags).NotTo(Ω.BeNil())
}

func TestReleaseNotes_Execute(t *testing.T) {
	please := Ω.NewWithT(t)

	t.Setenv("GITHUB_TOKEN", "")

	repo, _ := git.Init(memory.NewStorage(), memfs.New())

	revisionResolver := new(fakes.RevisionResolver)
	var (
		initialHash, finalHash plumbing.Hash
	)
	fill(initialHash[:], '1')
	fill(finalHash[:], '9')
	revisionResolver.ResolveRevisionReturnsOnCall(0, &initialHash, nil)
	revisionResolver.ResolveRevisionReturnsOnCall(1, &finalHash, nil)

	historicKilnfile := new(fakes.HistoricKilnfile)
	historicKilnfile.ReturnsOnCall(0, cargo.Kilnfile{}, cargo.KilnfileLock{
		Releases: []cargo.ComponentLock{
			{Name: "banana", Version: "1.1.0"},
			{Name: "lemon", Version: "1.1.0"},
		},
	}, nil)
	historicKilnfile.ReturnsOnCall(1, cargo.Kilnfile{
		Releases: []cargo.ComponentSpec{
			{Name: "banana", GitRepositories: []string{
				"https://github.com/cloudfoundry/banana-release",
				"https://github.com/pivotal-cf/lts-banana-release",
			}},
			{Name: "lemon"},
		},
	}, cargo.KilnfileLock{
		Releases: []cargo.ComponentLock{
			{Name: "banana", Version: "1.2.0"},
			{Name: "lemon", Version: "1.1.0"},
		},
	}, nil)

	historicVersion := new(fakes.HistoricVersion)
	historicVersion.Returns("0.1.0-build.50000", nil)

	readFileCount := 0
	readFileFunc := func(string) ([]byte, error) {
		readFileCount++
		return nil, nil
	}

	githubAPIIssuesServiceFake := new(fakes.GithubAPIIssuesService)
	githubAPIIssuesServiceFake.GetReturnsOnCall(0, &github.Issue{
		Title: strPtr("**[Feature Improvement]** Reduce default log-cache max per source"),
	}, githubResponse(t, 200), nil)
	githubAPIIssuesServiceFake.ListByRepoReturnsOnCall(1, []*github.Issue{}, githubResponse(t, 404), nil)
	githubAPIIssuesServiceFake.GetReturnsOnCall(0, &github.Issue{
		ID:    int64Ptr(1),
		Title: strPtr("**[Bug Fix]** banana metadata migration does not fail on upgrade from previous LTS"),
	}, githubResponse(t, 200), nil)
	githubAPIIssuesServiceFake.GetReturnsOnCall(1, &github.Issue{
		ID:    int64Ptr(2),
		Title: strPtr("**[Feature Improvement]** Reduce default log-cache max per source"),
	}, githubResponse(t, 200), nil)

	releaseListerFake := new(fakes.ReleaseLister)
	releaseListerFake.ListReleasesReturnsOnCall(0, []*github.RepositoryRelease{
		{TagName: strPtr("1.1.0"), Body: strPtr("   peal is green\n")},
		{TagName: strPtr("1.2.0"), Body: strPtr("peal is yellow")},
	}, githubResponse(t, 200), nil)
	releaseListerFake.ListReleasesReturnsOnCall(1, []*github.RepositoryRelease{}, githubResponse(t, 400), nil)
	releaseListerFake.ListReleasesReturnsOnCall(2, []*github.RepositoryRelease{
		{TagName: strPtr("1.1.0"), Body: strPtr("   peal is green\n")},
		{TagName: strPtr("1.1.1"), Body: strPtr("remove from bunch\n\n")},
		{TagName: strPtr("1.2.0"), Body: strPtr("peal is yellow")},
	}, githubResponse(t, 200), nil)
	releaseListerFake.ListReleasesReturnsOnCall(3, []*github.RepositoryRelease{}, githubResponse(t, 400), nil)

	var gotToken []string
	var output bytes.Buffer
	rn := ReleaseNotes{
		Writer:    &output,
		repoOwner: "pivotal-cf",
		repoName:  "fake-tile-repo",

		repository:       repo,
		revisionResolver: revisionResolver,
		historicKilnfile: historicKilnfile.Spy,
		historicVersion:  historicVersion.Spy,
		readFile:         readFileFunc,
		gitHubAPIServices: func(ctx context.Context, token string) (githubAPIIssuesService, releaseLister) {
			gotToken = append(gotToken, token)
			return githubAPIIssuesServiceFake, releaseListerFake
		},
	}

	err := rn.Execute([]string{
		"--release-date=2021-11-04",
		"--github-token=secret",
		"--github-issue=54000",
		"--github-issue=54321",
		"tile/1.1.0",
		"tile/1.2.0",
	})
	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(revisionResolver.ResolveRevisionCallCount()).To(Ω.Equal(2))
	please.Expect(revisionResolver.ResolveRevisionArgsForCall(0)).To(Ω.Equal(plumbing.Revision("tile/1.1.0")))
	please.Expect(revisionResolver.ResolveRevisionArgsForCall(1)).To(Ω.Equal(plumbing.Revision("tile/1.2.0")))

	please.Expect(historicVersion.CallCount()).To(Ω.Equal(1))
	_, historicVersionHashArg, _ := historicVersion.ArgsForCall(0)
	please.Expect(historicVersionHashArg).To(Ω.Equal(finalHash))
	please.Expect(gotToken).To(Ω.Equal([]string{"secret"}))
	please.Expect(releaseListerFake.ListReleasesCallCount()).To(Ω.Equal(4))

	please.Expect(githubAPIIssuesServiceFake.GetCallCount()).To(Ω.Equal(2))
	_, orgName, repoName, n := githubAPIIssuesServiceFake.GetArgsForCall(0)
	please.Expect(orgName).To(Ω.Equal("pivotal-cf"))
	please.Expect(repoName).To(Ω.Equal("fake-tile-repo"))
	please.Expect(n).To(Ω.Equal(54000))
	_, orgName, repoName, n = githubAPIIssuesServiceFake.GetArgsForCall(1)
	please.Expect(orgName).To(Ω.Equal("pivotal-cf"))
	please.Expect(repoName).To(Ω.Equal("fake-tile-repo"))
	please.Expect(n).To(Ω.Equal(54321))

	please.Expect(readFileCount).To(Ω.Equal(0))
	expected, err := ioutil.ReadFile("testdata/release_notes_output.md")
	please.Expect(err).NotTo(Ω.HaveOccurred())
	//t.Logf("got: %s", output.String())
	//t.Logf("exp: %s", expected)
	please.Expect(output.String()).To(Ω.Equal(string(expected)))
}

func TestReleaseNotes_Options_IssueTitleExp(t *testing.T) {
	please := Ω.NewWithT(t)
	expStr := getIssueTitleExp(t)
	please.Expect(expStr).NotTo(Ω.BeEmpty())
	exp, err := regexp.Compile(expStr)
	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(exp.MatchString("**[Bug Fix]** Lorem Ipsum")).To(Ω.BeTrue())
	please.Expect(exp.MatchString("**[bug fix]** Lorem Ipsum")).To(Ω.BeTrue())
	please.Expect(exp.MatchString("**[Feature]** Lorem Ipsum")).To(Ω.BeTrue())
	please.Expect(exp.MatchString("**[feature improvement]** Lorem Ipsum")).To(Ω.BeTrue())
	please.Expect(exp.MatchString("**[security fix]** Lorem Ipsum")).To(Ω.BeTrue())

	please.Expect(exp.MatchString("**[none]** Lorem Ipsum")).To(Ω.BeFalse())
	please.Expect(exp.MatchString("Lorem Ipsum")).To(Ω.BeFalse())
	please.Expect(exp.MatchString("")).To(Ω.BeFalse())
	please.Expect(exp.MatchString("**[]**")).To(Ω.BeFalse())
	please.Expect(exp.MatchString("**[bugFix]**")).To(Ω.BeFalse())
	please.Expect(exp.MatchString("**[security]**")).To(Ω.BeFalse())
}

func getIssueTitleExp(t *testing.T) string {
	t.Helper()
	issueTitleExpField, ok := reflect.TypeOf(ReleaseNotes{}.Options).FieldByName("IssueTitleExp")
	if !ok {
		t.Fatal("failed to get field")
	}
	return issueTitleExpField.Tag.Get("default")
}

func fill(buf []byte, value byte) {
	for i := range buf {
		buf[i] = value
	}
}
