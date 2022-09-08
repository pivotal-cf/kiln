package notes

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sort"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/pkg/notes/fakes"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v40/github"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func Test_fetch(t *testing.T) {
	please := NewWithT(t)

	t.Setenv("GITHUB_TOKEN", "")

	repo, _ := git.Init(memory.NewStorage(), memfs.New())

	revisionResolver := new(fakes.RevisionResolver)
	var initialHash, finalHash plumbing.Hash
	fill(initialHash[:], '1')
	fill(finalHash[:], '9')
	revisionResolver.ResolveRevisionReturnsOnCall(0, &initialHash, nil)
	revisionResolver.ResolveRevisionReturnsOnCall(1, &finalHash, nil)

	historicKilnfile := new(fakes.HistoricKilnfile)
	historicKilnfile.ReturnsOnCall(0, cargo.Kilnfile{}, cargo.KilnfileLock{
		Stemcell: cargo.Stemcell{
			OS:      "fruit-tree",
			Version: "40000.1",
		},
		Releases: []cargo.ReleaseLock{
			{Name: "banana", Version: "1.1.0"},
			{Name: "lemon", Version: "1.1.0"},
		},
	}, nil)
	historicKilnfile.ReturnsOnCall(1, cargo.Kilnfile{
		Stemcell: cargo.Stemcell{
			OS:      "fruit-tree",
			Version: "40000.2",
		},
		Releases: []cargo.ReleaseSpec{
			{Name: "banana", GitHubRepository: "https://github.com/pivotal-cf/lts-banana-release"},
			{Name: "lemon"},
		},
	}, cargo.KilnfileLock{
		Releases: []cargo.ReleaseLock{
			{Name: "banana", Version: "1.2.0"},
			{Name: "lemon", Version: "1.1.0"},
		},
	}, nil)

	historicVersion := new(fakes.HistoricVersion)
	historicVersion.Returns("0.1.0-build.50000", nil)

	fakeIssuesService := new(fakes.IssuesService)
	fakeIssuesService.GetReturnsOnCall(0, &github.Issue{
		Title: strPtr("**[Feature Improvement]** Reduce default log-cache max per source"),
	}, githubResponse(t, 200), nil)
	fakeIssuesService.ListByRepoReturnsOnCall(1, []*github.Issue{}, githubResponse(t, 404), nil)
	fakeIssuesService.GetReturnsOnCall(0, &github.Issue{
		ID:    int64Ptr(1),
		Title: strPtr("**[Bug Fix]** banana metadata migration does not fail on upgrade from previous LTS"),
	}, githubResponse(t, 200), nil)
	fakeIssuesService.GetReturnsOnCall(1, &github.Issue{
		ID:    int64Ptr(2),
		Title: strPtr("**[Feature Improvement]** Reduce default log-cache max per source"),
	}, githubResponse(t, 200), nil)

	fakeReleaseService := new(fakes.RepositoryReleaseLister)
	fakeReleaseService.ListReleasesReturnsOnCall(0, []*github.RepositoryRelease{
		{TagName: strPtr("1.1.0"), Body: strPtr("   peal is green\n")},
		{TagName: strPtr("1.1.1"), Body: strPtr("remove from bunch\n\n")},
	}, githubResponse(t, 200), nil)
	fakeReleaseService.ListReleasesReturnsOnCall(2, []*github.RepositoryRelease{
		{TagName: strPtr("1.1.2"), Body: strPtr("")},
		{TagName: strPtr("1.2.0"), Body: strPtr("peal is yellow")},
	}, githubResponse(t, 200), nil)
	fakeReleaseService.ListReleasesReturnsOnCall(3, []*github.RepositoryRelease{}, githubResponse(t, 400), nil)

	rn := fetchNotesData{
		repoOwner:       "pivotal-cf",
		repoName:        "fake-tile-repo",
		kilnfilePath:    "tile",
		initialRevision: "tile/1.1.0",
		finalRevision:   "tile/1.2.0",
		issuesQuery: IssuesQuery{
			IssueIDs: []string{"54000", "54321"},
		},

		repository:       repo,
		Storer:           repo.Storer,
		revisionResolver: revisionResolver,
		historicKilnfile: historicKilnfile.Spy,
		historicVersion:  historicVersion.Spy,

		issuesService:   fakeIssuesService,
		releasesService: fakeReleaseService,
	}

	_, err := rn.fetch(context.Background())
	please.Expect(err).NotTo(HaveOccurred())

	please.Expect(revisionResolver.ResolveRevisionCallCount()).To(Equal(2))
	please.Expect(revisionResolver.ResolveRevisionArgsForCall(0)).To(Equal(plumbing.Revision("tile/1.1.0")))
	please.Expect(revisionResolver.ResolveRevisionArgsForCall(1)).To(Equal(plumbing.Revision("tile/1.2.0")))

	please.Expect(historicVersion.CallCount()).To(Equal(1))
	_, historicVersionHashArg, _ := historicVersion.ArgsForCall(0)
	please.Expect(historicVersionHashArg).To(Equal(finalHash))
	please.Expect(fakeReleaseService.ListReleasesCallCount()).To(Equal(2))
	please.Expect(fakeIssuesService.GetCallCount()).To(Equal(2))

	_, orgName, repoName, n := fakeIssuesService.GetArgsForCall(0)
	please.Expect(orgName).To(Equal("pivotal-cf"))
	please.Expect(repoName).To(Equal("fake-tile-repo"))
	please.Expect(n).To(Equal(54000))

	_, orgName, repoName, n = fakeIssuesService.GetArgsForCall(1)
	please.Expect(orgName).To(Equal("pivotal-cf"))
	please.Expect(repoName).To(Equal("fake-tile-repo"))
	please.Expect(n).To(Equal(54321))
}

func Test_issuesFromIssueIDs(t *testing.T) {
	t.Parallel()

	t.Run("no ids", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.IssueGetter)

		result, err := issuesFromIssueIDs(context.Background(), issuesService, "o", "n", nil)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(result).To(HaveLen(0))
		please.Expect(issuesService.GetCallCount()).To(Equal(0))
	})

	t.Run("some ids", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.IssueGetter)

		issuesService.GetReturnsOnCall(0, &github.Issue{Number: intPtr(1)}, githubResponse(t, 200), nil)
		issuesService.GetReturnsOnCall(1, &github.Issue{Number: intPtr(2)}, githubResponse(t, 200), nil)

		result, err := issuesFromIssueIDs(context.Background(), issuesService, "o", "n", []string{"1", "2"})
		please.Expect(err).NotTo(HaveOccurred())

		please.Expect(result).To(HaveLen(2))
		please.Expect(result[0].GetNumber()).To(Equal(1))
		please.Expect(result[1].GetNumber()).To(Equal(2))

		please.Expect(issuesService.GetCallCount()).To(Equal(2))
		ctx, ro, rn, number := issuesService.GetArgsForCall(0)
		please.Expect(ctx).NotTo(BeNil())
		please.Expect(ro).To(Equal("o"))
		please.Expect(rn).To(Equal("n"))
		please.Expect(number).To(Equal(1))

		_, _, _, number = issuesService.GetArgsForCall(1)
		please.Expect(number).To(Equal(2))
	})

	t.Run("the issues service returns an error", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.IssueGetter)

		issuesService.GetReturnsOnCall(0, &github.Issue{Number: intPtr(1)}, nil, errors.New("banana"))

		_, err := issuesFromIssueIDs(context.Background(), issuesService, "o", "n", []string{"1"})
		please.Expect(err).To(HaveOccurred())
	})

	t.Run("the issues service returns a not okay status", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.IssueGetter)

		issuesService.GetReturnsOnCall(0, &github.Issue{Number: intPtr(1)}, githubResponse(t, http.StatusUnauthorized), nil)

		_, err := issuesFromIssueIDs(context.Background(), issuesService, "o", "n", []string{"1"})
		please.Expect(err).To(HaveOccurred())
	})
}

func Test_resolveMilestoneNumber(t *testing.T) {
	t.Parallel()

	t.Run("empty milestone option", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		result, err := resolveMilestoneNumber(context.Background(), issuesService, "o", "n", "")
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(result).To(Equal(""))
	})

	t.Run("when passed a number", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		result, err := resolveMilestoneNumber(context.Background(), issuesService, "o", "n", "42")
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(result).To(Equal("42"), "it returns that number")
		please.Expect(issuesService.ListMilestonesCallCount()).To(Equal(0), "it does not reach out o")
	})

	t.Run("when the milestone is found on the second page", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		issuesService.ListMilestonesReturnsOnCall(0,
			[]*github.Milestone{
				{Title: strPtr("orange")},
				{Title: strPtr("lemon")},
			},
			githubResponse(t, 200),
			nil,
		)
		issuesService.ListMilestonesReturnsOnCall(1,
			[]*github.Milestone{
				{Title: strPtr("kiwi")},
				{Title: strPtr("banana"), Number: intPtr(42)},
			},
			githubResponse(t, 200),
			nil,
		)

		result, err := resolveMilestoneNumber(context.Background(), issuesService, "o", "n", "banana")

		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(issuesService.ListMilestonesCallCount()).To(Equal(2))
		please.Expect(result).To(Equal("42"))
	})

	// TODO: test pagination

	t.Run("the issues service returns an error", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		issuesService.ListMilestonesReturns(nil, nil, errors.New("banana"))

		_, err := resolveMilestoneNumber(context.Background(), issuesService, "o", "n", "m")
		please.Expect(err).To(HaveOccurred())
	})

	t.Run("the issues service returns a not okay status", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		issuesService.ListMilestonesReturns(nil, githubResponse(t, http.StatusUnauthorized), nil)

		_, err := resolveMilestoneNumber(context.Background(), issuesService, "o", "n", "m")
		please.Expect(err).To(HaveOccurred())
	})
}

func Test_fetchIssuesWithLabelAndMilestone(t *testing.T) {
	t.Parallel()

	t.Run("empty milestone and labels", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.IssuesByRepoLister)

		result, err := fetchIssuesWithLabelAndMilestone(context.Background(), issuesService, "o", "n", "", nil)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(result).To(HaveLen(0))
	})

	// TODO: issue service call params

	t.Run("the issues service returns an error", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.IssuesByRepoLister)

		issuesService.ListByRepoReturns(nil, nil, errors.New("banana"))

		_, err := fetchIssuesWithLabelAndMilestone(context.Background(), issuesService, "o", "n", "1", nil)
		please.Expect(err).To(HaveOccurred())
	})

	t.Run("the issues service returns a not okay status", func(t *testing.T) {
		please := NewWithT(t)
		issuesService := new(fakes.IssuesByRepoLister)

		issuesService.ListByRepoReturns(nil, githubResponse(t, http.StatusUnauthorized), nil)

		_, err := fetchIssuesWithLabelAndMilestone(context.Background(), issuesService, "o", "n", "1", nil)
		please.Expect(err).To(HaveOccurred())
	})
}

func Test_issuesBySemanticTitlePrefix(t *testing.T) {
	please := NewWithT(t)

	issues := []*github.Issue{
		{Title: strPtr("**[NONE]** lorem ipsum")},
		{Title: strPtr("**[security fix]** 222 lorem ipsum")},
		{Title: strPtr("**[Feature]** lorem ipsum")},
		{Title: strPtr("**[Feature Improvement]** lorem ipsum")},
		{Title: strPtr("**[security Fix]** 111 lorem ipsum")},
		{Title: strPtr("**[Bug Fix]** lorem ipsum")},
	}
	sort.Sort(issuesBySemanticTitlePrefix(issues))

	var titles []string
	for _, issue := range issues {
		titles = append(titles, issue.GetTitle())
	}

	please.Expect(titles).To(Equal([]string{
		"**[security Fix]** 111 lorem ipsum",
		"**[security fix]** 222 lorem ipsum",
		"**[Feature]** lorem ipsum",
		"**[Feature Improvement]** lorem ipsum",
		"**[Bug Fix]** lorem ipsum",
		"**[NONE]** lorem ipsum",
	}))
}

func Test_appendFilterAndSortIssues(t *testing.T) {
	please := NewWithT(t)
	getID := func() func() int64 {
		var n int64
		return func() int64 {
			n++
			return n
		}
	}()

	issues := []*github.Issue{
		{Title: strPtr("**[security fix]** 222 lorem ipsum"), ID: int64Ptr(getID())},
		{Title: strPtr("**[Feature]** lorem ipsum"), ID: int64Ptr(getID())},
		{Title: strPtr("**[Feature Improvement]** lorem ipsum"), ID: int64Ptr(getID())},
		{Title: strPtr("**[security Fix]** 111 lorem ipsum"), ID: int64Ptr(getID())},
		{Title: strPtr("**[Breaking Change]** 333 lorem ipsum"), ID: int64Ptr(getID())},
	}

	additionalIssues := []*github.Issue{
		{Title: strPtr("**[NONE]** lorem ipsum"), ID: int64Ptr(getID())},
		{Title: strPtr("**[Bug Fix]** lorem ipsum"), ID: int64Ptr(getID())},
	}
	exp := getIssueTitleExp(t)
	result := appendFilterAndSortIssues(issues, additionalIssues, exp)

	var titles []string
	for _, issue := range result {
		titles = append(titles, issue.GetTitle())
	}

	please.Expect(titles).To(Equal([]string{
		"**[security Fix]** 111 lorem ipsum",
		"**[security fix]** 222 lorem ipsum",
		"**[Feature]** lorem ipsum",
		"**[Feature Improvement]** lorem ipsum",
		"**[Bug Fix]** lorem ipsum",
		"**[Breaking Change]** 333 lorem ipsum",
	}))
}

func TestReleaseNotes_Options_IssueTitleExp(t *testing.T) {
	please := NewWithT(t)

	exp, err := IssuesQuery{}.Exp()
	please.Expect(err).NotTo(HaveOccurred())

	please.Expect(exp.MatchString("**[Bug Fix]** Lorem Ipsum")).To(BeTrue())
	please.Expect(exp.MatchString("**[bug fix]** Lorem Ipsum")).To(BeTrue())
	please.Expect(exp.MatchString("**[Feature]** Lorem Ipsum")).To(BeTrue())
	please.Expect(exp.MatchString("**[feature improvement]** Lorem Ipsum")).To(BeTrue())
	please.Expect(exp.MatchString("**[security fix]** Lorem Ipsum")).To(BeTrue())

	please.Expect(exp.MatchString("**[none]** Lorem Ipsum")).To(BeFalse())
	please.Expect(exp.MatchString("**[none]** feature bug fix security")).To(BeFalse())
	please.Expect(exp.MatchString("Lorem Ipsum")).To(BeFalse())
	please.Expect(exp.MatchString("")).To(BeFalse())
	please.Expect(exp.MatchString("**[]**")).To(BeFalse())
	please.Expect(exp.MatchString("**[bugFix]**")).To(BeFalse())
	please.Expect(exp.MatchString("**[security]**")).To(BeFalse())
}

func getIssueTitleExp(t *testing.T) string {
	t.Helper()
	issueTitleExpField, ok := reflect.TypeOf(IssuesQuery{}).FieldByName("IssueTitleExp")
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

func githubResponse(t *testing.T, status int) *github.Response {
	t.Helper()

	return &github.Response{
		Response: &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
		},
	}
}

func intPtr(n int) *int       { return &n }
func int64Ptr(n int64) *int64 { return &n }
func strPtr(n string) *string { return &n }
