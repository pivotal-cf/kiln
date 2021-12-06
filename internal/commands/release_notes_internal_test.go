package commands

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"sort"
	"testing"

	Ω "github.com/onsi/gomega"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v40/github"

	fakes "github.com/pivotal-cf/kiln/internal/commands/fakes_internal"
)

func TestInternal_ReleaseNotes_checkInputs(t *testing.T) {
	t.Parallel()

	t.Run("missing args", func(t *testing.T) {
		please := Ω.NewWithT(t)

		rn := ReleaseNotes{}
		err := rn.checkInputs(nil)
		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("expected two arguments")))
	})

	t.Run("missing arg", func(t *testing.T) {
		please := Ω.NewWithT(t)

		rn := ReleaseNotes{}
		err := rn.checkInputs([]string{"some-hash"})
		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("expected two arguments")))
	})

	t.Run("too many args", func(t *testing.T) {
		please := Ω.NewWithT(t)

		rn := ReleaseNotes{}
		err := rn.checkInputs([]string{"a", "b", "c"})
		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("expected two arguments")))
	})

	t.Run("too many args", func(t *testing.T) {
		please := Ω.NewWithT(t)

		rn := ReleaseNotes{}
		err := rn.checkInputs([]string{"a", "b", "c"})
		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("expected two arguments")))
	})

	t.Run("bad issue title expression", func(t *testing.T) {
		please := Ω.NewWithT(t)

		rn := ReleaseNotes{}
		rn.Options.IssueTitleExp = `\`
		err := rn.checkInputs([]string{"a", "b"})
		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("expression")))
	})

	t.Run("malformed release date", func(t *testing.T) {
		please := Ω.NewWithT(t)

		rn := ReleaseNotes{}
		rn.Options.ReleaseDate = `some-date`
		err := rn.checkInputs([]string{"a", "b"})
		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("cannot parse")))
	})

	t.Run("issue flag without auth", func(t *testing.T) {
		t.Run("milestone", func(t *testing.T) {
			please := Ω.NewWithT(t)

			rn := ReleaseNotes{}
			rn.Options.IssueMilestone = "s"
			err := rn.checkInputs([]string{"a", "b"})
			please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("github-token")))
		})

		t.Run("ids", func(t *testing.T) {
			please := Ω.NewWithT(t)

			rn := ReleaseNotes{}
			rn.Options.IssueIDs = []string{"s"}
			err := rn.checkInputs([]string{"a", "b"})
			please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("github-token")))
		})

		t.Run("labels", func(t *testing.T) {
			please := Ω.NewWithT(t)

			rn := ReleaseNotes{}
			rn.Options.IssueLabels = []string{"s"}
			err := rn.checkInputs([]string{"a", "b"})
			please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("github-token")))
		})

		t.Run("exp", func(t *testing.T) {
			please := Ω.NewWithT(t)

			rn := ReleaseNotes{}
			rn.Options.IssueTitleExp = "s"
			err := rn.checkInputs([]string{"a", "b"})
			please.Expect(err).NotTo(Ω.HaveOccurred())
		})
	})
}

func TestInternal_getGithubRemoteRepoOwnerAndName(t *testing.T) {
	t.Parallel()
	t.Run("when there is a github http remote", func(t *testing.T) {
		please := Ω.NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"https://github.com/pivotal-cf/kiln",
			},
		})
		o, r, err := getGithubRemoteRepoOwnerAndName(repo)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(o).To(Ω.Equal("pivotal-cf"))
		please.Expect(r).To(Ω.Equal("kiln"))
	})

	t.Run("when there is a github ssh remote", func(t *testing.T) {
		please := Ω.NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"git@github.com:pivotal-cf/kiln.git",
			},
		})
		o, r, err := getGithubRemoteRepoOwnerAndName(repo)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(o).To(Ω.Equal("pivotal-cf"))
		please.Expect(r).To(Ω.Equal("kiln"))
	})

	t.Run("when there are no remotes", func(t *testing.T) {
		please := Ω.NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _, err := getGithubRemoteRepoOwnerAndName(repo)
		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("not found")))
	})

	t.Run("when there are many remotes", func(t *testing.T) {
		please := Ω.NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "fork",
			URLs: []string{
				"git@github.com:crhntr/kiln.git",
			},
		})
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"git@github.com:pivotal-cf/kiln.git",
			},
		})
		o, _, err := getGithubRemoteRepoOwnerAndName(repo)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(o).To(Ω.Equal("pivotal-cf"), "it uses the remote with name 'origin'")
	})
}

func TestInternal_issuesFromIssueIDs(t *testing.T) {
	t.Parallel()

	t.Run("no ids", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.IssueGetter)

		result, err := issuesFromIssueIDs(context.TODO(), issuesService, "o", "n", nil)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(result).To(Ω.HaveLen(0))
		please.Expect(issuesService.GetCallCount()).To(Ω.Equal(0))
	})

	t.Run("some ids", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.IssueGetter)

		issuesService.GetReturnsOnCall(0, &github.Issue{Number: intPtr(1)}, githubResponse(t, 200), nil)
		issuesService.GetReturnsOnCall(1, &github.Issue{Number: intPtr(2)}, githubResponse(t, 200), nil)

		result, err := issuesFromIssueIDs(context.TODO(), issuesService, "o", "n", []string{"1", "2"})
		please.Expect(err).NotTo(Ω.HaveOccurred())

		please.Expect(result).To(Ω.HaveLen(2))
		please.Expect(result[0].GetNumber()).To(Ω.Equal(1))
		please.Expect(result[1].GetNumber()).To(Ω.Equal(2))

		please.Expect(issuesService.GetCallCount()).To(Ω.Equal(2))
		ctx, ro, rn, number := issuesService.GetArgsForCall(0)
		please.Expect(ctx).NotTo(Ω.BeNil())
		please.Expect(ro).To(Ω.Equal("o"))
		please.Expect(rn).To(Ω.Equal("n"))
		please.Expect(number).To(Ω.Equal(1))

		_, _, _, number = issuesService.GetArgsForCall(1)
		please.Expect(number).To(Ω.Equal(2))
	})

	t.Run("the issues service returns an error", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.IssueGetter)

		issuesService.GetReturnsOnCall(0, &github.Issue{Number: intPtr(1)}, nil, errors.New("banana"))

		_, err := issuesFromIssueIDs(context.TODO(), issuesService, "o", "n", []string{"1"})
		please.Expect(err).To(Ω.HaveOccurred())
	})

	t.Run("the issues service returns a not okay status", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.IssueGetter)

		issuesService.GetReturnsOnCall(0, &github.Issue{Number: intPtr(1)}, githubResponse(t, http.StatusUnauthorized), nil)

		_, err := issuesFromIssueIDs(context.TODO(), issuesService, "o", "n", []string{"1"})
		please.Expect(err).To(Ω.HaveOccurred())
	})
}

func TestInternal_resolveMilestoneNumber(t *testing.T) {
	t.Parallel()

	t.Run("empty milestone option", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		result, err := resolveMilestoneNumber(context.TODO(), issuesService, "o", "n", "")
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(result).To(Ω.Equal(""))
	})

	t.Run("when passed a number", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		result, err := resolveMilestoneNumber(context.TODO(), issuesService, "o", "n", "42")
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(result).To(Ω.Equal("42"), "it returns that number")
		please.Expect(issuesService.ListMilestonesCallCount()).To(Ω.Equal(0), "it does not reach out o")
	})

	t.Run("when the milestone is found on the second page", func(t *testing.T) {
		please := Ω.NewWithT(t)
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

		result, err := resolveMilestoneNumber(context.TODO(), issuesService, "o", "n", "banana")

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(issuesService.ListMilestonesCallCount()).To(Ω.Equal(2))
		please.Expect(result).To(Ω.Equal("42"))
	})

	// TODO: test pagination

	t.Run("the issues service returns an error", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		issuesService.ListMilestonesReturns(nil, nil, errors.New("banana"))

		_, err := resolveMilestoneNumber(context.TODO(), issuesService, "o", "n", "m")
		please.Expect(err).To(Ω.HaveOccurred())
	})

	t.Run("the issues service returns a not okay status", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.MilestoneLister)

		issuesService.ListMilestonesReturns(nil, githubResponse(t, http.StatusUnauthorized), nil)

		_, err := resolveMilestoneNumber(context.TODO(), issuesService, "o", "n", "m")
		please.Expect(err).To(Ω.HaveOccurred())
	})
}

func TestInternal_fetchIssuesWithLabelAndMilestone(t *testing.T) {
	t.Parallel()

	t.Run("empty milestone and labels", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.IssuesByRepoLister)

		result, err := fetchIssuesWithLabelAndMilestone(context.TODO(), issuesService, "o", "n", "", nil)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(result).To(Ω.HaveLen(0))
	})

	// TODO: issue service call params

	t.Run("the issues service returns an error", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.IssuesByRepoLister)

		issuesService.ListByRepoReturns(nil, nil, errors.New("banana"))

		_, err := fetchIssuesWithLabelAndMilestone(context.TODO(), issuesService, "o", "n", "1", nil)
		please.Expect(err).To(Ω.HaveOccurred())
	})

	t.Run("the issues service returns a not okay status", func(t *testing.T) {
		please := Ω.NewWithT(t)
		issuesService := new(fakes.IssuesByRepoLister)

		issuesService.ListByRepoReturns(nil, githubResponse(t, http.StatusUnauthorized), nil)

		_, err := fetchIssuesWithLabelAndMilestone(context.TODO(), issuesService, "o", "n", "1", nil)
		please.Expect(err).To(Ω.HaveOccurred())
	})
}

func TestInternal_issuesBySemanticTitlePrefix(t *testing.T) {
	please := Ω.NewWithT(t)

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

	please.Expect(titles).To(Ω.Equal([]string{
		"**[security Fix]** 111 lorem ipsum",
		"**[security fix]** 222 lorem ipsum",
		"**[Feature]** lorem ipsum",
		"**[Feature Improvement]** lorem ipsum",
		"**[Bug Fix]** lorem ipsum",
		"**[NONE]** lorem ipsum",
	}))
}

func TestInternal_appendFilterAndSortIssues(t *testing.T) {
	please := Ω.NewWithT(t)
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

	please.Expect(titles).To(Ω.Equal([]string{
		"**[security Fix]** 111 lorem ipsum",
		"**[security fix]** 222 lorem ipsum",
		"**[Feature]** lorem ipsum",
		"**[Feature Improvement]** lorem ipsum",
		"**[Bug Fix]** lorem ipsum",
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

func intPtr(n int) *int       { return &n }
func int64Ptr(n int64) *int64 { return &n }
func strPtr(n string) *string { return &n }
