package commands

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v40/github"
	"github.com/pivotal-cf/jhanda"
	"golang.org/x/oauth2"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/historic"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const releaseDateFormat = "2006-01-02"

type ReleaseNotes struct {
	Options struct {
		ReleaseDate    string   `long:"release-date"           short:"d" description:"release date of the tile"`
		TemplateName   string   `long:"template"               short:"t" description:"path to template"`
		GithubToken    string   `long:"github-token"           short:"g" description:"auth token for fetching issues merged between releases" env:"GITHUB_TOKEN"`
		IssueIDs       []string `long:"github-issue"           short:"i" description:"a list of issues to include in the release notes; these are deduplicated with the issue results"`
		IssueMilestone string   `long:"github-issue-milestone" short:"m" description:"issue milestone to use, it may be the milestone number or the milestone name"`
		IssueLabels    []string `long:"github-issue-label"     short:"l" description:"issue labels to add to issues query"`
		IssueTitleExp  string   `long:"issues-title-exp"       short:"x" description:"issues with title matching regular expression will be added. Issues must first be fetched with github-issue* flags. The default expression can be disabled by setting an empty string" default:"(?i)^\\*\\*\\[(security fix)|(feature)|(feature improvement)|(bug fix)\\]\\*\\*.*$"`
	}

	pathRelativeToDotGit string
	Repository           *git.Repository
	ReadFile             func(fp string) ([]byte, error)
	HistoricKilnfileLock
	HistoricVersion
	RevisionResolver
	GetIssueFunc
	Stat func(string) (os.FileInfo, error)
	io.Writer

	RepoOwner, RepoName string
}

func NewReleaseNotesCommand() (ReleaseNotes, error) {
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return ReleaseNotes{}, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return ReleaseNotes{}, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return ReleaseNotes{}, err
	}
	rp, err := filepath.Rel(wt.Filesystem.Root(), wd)
	if err != nil {
		return ReleaseNotes{}, err
	}

	repoOwner, repoName, err := getGithubRemoteRepoOwnerAndName(repo)
	if err != nil {
		return ReleaseNotes{}, err
	}

	return ReleaseNotes{
		Repository:               repo,
		ReadFile:                 ioutil.ReadFile,
		HistoricKilnfileLockFunc: historic.KilnfileLock,
		HistoricVersionFunc:      historic.Version,
		RevisionResolver:         repo,
		Stat:                     os.Stat,
		Writer:                   os.Stdout,
		pathRelativeToDotGit:     rp,
		RepoName:                 repoName,
		RepoOwner:                repoOwner,
	}, nil
}

//counterfeiter:generate -o ./fakes/historic_version.go --fake-name HistoricVersion . HistoricVersion

type HistoricVersion func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (string, error)

//counterfeiter:generate -o ./fakes/historic_kilnfile_lock_func.go --fake-name HistoricKilnfileLock . HistoricKilnfileLock

type HistoricKilnfileLock func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (cargo.KilnfileLock, error)

//counterfeiter:generate -o ./fakes/get_github_issue.go --fake-name GetGithubIssue . GetGithubIssue

type GetIssueFunc func(ctx context.Context, owner string, repo string, number int) (*github.Issue, *github.Response, error)

//counterfeiter:generate -o ./fakes/revision_resolver.go --fake-name RevisionResolver . RevisionResolver

type RevisionResolver interface {
	ResolveRevision(rev plumbing.Revision) (*plumbing.Hash, error)
}

//go:embed release_notes.md.template
var defaultReleaseNotesTemplate string

func (r ReleaseNotes) Execute(args []string) error {
	nonFlagArgs, err := jhanda.Parse(&r.Options, args) // TODO handle error
	if err != nil {
		return err
	}
	err = r.checkInputs(nonFlagArgs)
	if err != nil {
		return err
	}

	releaseDate, err := parseReleaseDate(r.Options.ReleaseDate)
	if err != nil {
		return err
	}

	initialCommitSHA, err := r.ResolveRevision(plumbing.Revision(nonFlagArgs[0])) // TODO handle error
	if err != nil {
		panic(err)
	}
	finalCommitSHA, err := r.ResolveRevision(plumbing.Revision(nonFlagArgs[1])) // TODO handle error
	if err != nil {
		panic(err)
	}

	klInitial, err := r.HistoricKilnfileLock(r.Repository, *initialCommitSHA, r.pathRelativeToDotGit) // TODO handle error
	if err != nil {
		panic(err)
	}
	klFinal, err := r.HistoricKilnfileLock(r.Repository, *finalCommitSHA, r.pathRelativeToDotGit) // TODO handle error
	if err != nil {
		panic(err)
	}
	version, err := r.HistoricVersion(r.Repository, *finalCommitSHA, r.pathRelativeToDotGit) // TODO handle error
	if err != nil {
		panic(err)
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return fmt.Errorf("failed to parse version: %w", err)
	}

	info := ReleaseNotesInformation{
		Version:           v,
		ReleaseDate:       releaseDate,
		ReleaseDateFormat: releaseDateFormat,
		Components:        klFinal.Releases,
		Bumps:             calculateComponentBumps(klFinal.Releases, klInitial.Releases),
	}

	info.Issues, err = r.listGithubIssues(context.TODO())
	if err != nil {
		return err
	}

	releaseNotesTemplate := defaultReleaseNotesTemplate
	if r.Options.TemplateName != "" {
		templateBuf, _ := r.ReadFile(r.Options.TemplateName) // TODO handle error
		releaseNotesTemplate = string(templateBuf)
	}

	t, err := template.New(r.Options.TemplateName).Parse(releaseNotesTemplate) // TODO handle error
	if err != nil {
		panic(err)
	}

	err = t.Execute(r.Writer, info)
	if err != nil {
		return err
	}

	return nil
}

func (r ReleaseNotes) parseReleaseDate() (time.Time, error) {
	var releaseDate time.Time

	if r.Options.ReleaseDate != "" {
		var err error
		releaseDate, err = time.Parse(releaseDateFormat, r.Options.ReleaseDate)
		if err != nil {
			return time.Time{}, fmt.Errorf("release date could not be parsed: %w", err)
		}
	}

	return releaseDate, nil
}

func (r ReleaseNotes) checkInputs(nonFlagArgs []string) error {
	if len(nonFlagArgs) != 2 {
		return errors.New("expected two arguments: <Git-Revision> <Git-Revision>")
	}

	if r.Options.IssueTitleExp != "" {
		_, err := regexp.Compile(r.Options.IssueTitleExp)
		if err != nil {
			return fmt.Errorf("failed to parse issues-title-exp: %w", err)
		}
	}

	if r.Options.GithubToken == "" &&
		(r.Options.IssueTitleExp != "" ||
			r.Options.IssueMilestone != "" ||
			len(r.Options.IssueIDs) > 0 ||
			len(r.Options.IssueLabels) > 0) {
		return errors.New("github-token (env: GITHUB_TOKEN) must be set to interact with the github api")
	}

	_, err := parseReleaseDate(r.Options.ReleaseDate)
	if err != nil {
		return err
	}

	return nil
}

func getGithubRemoteRepoOwnerAndName(repo *git.Repository) (string, string, error) {
	var remoteURL string
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", err
	}
	config := remote.Config()
	for _, u := range config.URLs {
		if !strings.Contains(u, "github.com") {
			continue
		}
		remoteURL = u
		break
	}
	if remoteURL == "" {
		return "", "", fmt.Errorf("remote github URL not found for repo")
	}

	repoOwner, repoName := component.OwnerAndRepoFromGitHubURI(remoteURL)
	if repoOwner == "" || repoName == "" {
		return "", "", errors.New("could not determine owner and repo for tile")
	}

	return repoOwner, repoName, nil
}

type ReleaseNotesInformation struct {
	Version           *semver.Version
	ReleaseDate       time.Time
	ReleaseDateFormat string

	Issues []*github.Issue

	Bumps      []component.Lock
	Components []component.Lock
}

type BoshReleaseBump = component.Lock

func calculateComponentBumps(current, previous []component.Lock) []BoshReleaseBump {
	var (
		bumps         []BoshReleaseBump
		previousSpecs = make(map[component.Lock]struct{}, len(previous))
	)
	for _, cs := range previous {
		previousSpecs[cs] = struct{}{}
	}
	for _, cs := range current {
		_, isSame := previousSpecs[cs]
		if isSame {
			continue
		}
		bumps = append(bumps, cs)
	}
	return bumps
}

// listGithubIssues is not tested. By not testing we are getting reduced abstraction and improved readability
// at the cost of high level testing. This function therefore must stay as small as possible and rely on type checking
// and a manual test to ensure it continues to behave as expected during refactors.
//
// The function can be tested by generating release notes for a tile with issue ids and a milestone set.
// The happy path test for Execute does not set GithubToken intentionally so this code is not triggered and Execute
// does not actually reach out to GitHub.
func (r ReleaseNotes) listGithubIssues(ctx context.Context) ([]*github.Issue, error) {
	if r.Options.GithubToken == "" {
		return nil, nil
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: r.Options.GithubToken})
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(tokenClient)

	milestoneNumber, err := resolveMilestoneNumber(ctx, githubClient.Issues, r.RepoOwner, r.RepoName, r.Options.IssueMilestone)
	if err != nil {
		return nil, err
	}
	issues, err := fetchIssuesWithLabelAndMilestone(ctx, githubClient.Issues, r.RepoOwner, r.RepoName, milestoneNumber, r.Options.IssueLabels)
	if err != nil {
		return nil, err
	}
	additionalIssues, err := issuesFromIssueIDs(ctx, githubClient.Issues, r.RepoOwner, r.RepoName, r.Options.IssueIDs)
	if err != nil {
		return nil, err
	}

	return appendFilterAndSortIssues(issues, additionalIssues, r.Options.IssueTitleExp), nil
}

//counterfeiter:generate -o ./fakes_internal/issue_getter.go --fake-name IssueGetter . issueGetter

type issueGetter interface {
	Get(ctx context.Context, owner string, repo string, number int) (*github.Issue, *github.Response, error)
}

func issuesFromIssueIDs(ctx context.Context, issuesService issueGetter, repoOwner, repoName string, issueIDs []string) ([]*github.Issue, error) {
	var issues []*github.Issue
	for _, id := range issueIDs {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("failed to parse issue id %q: %w", id, err)
		}
		issue, response, err := issuesService.Get(ctx, repoOwner, repoName, n)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to get issue %q: %w", id, err)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

//counterfeiter:generate -o ./fakes_internal/milestone_lister.go --fake-name MilestoneLister . milestoneLister

type milestoneLister interface {
	ListMilestones(ctx context.Context, owner string, repo string, opts *github.MilestoneListOptions) ([]*github.Milestone, *github.Response, error)
}

func resolveMilestoneNumber(ctx context.Context, issuesService milestoneLister, repoOwner, repoName, milestone string) (string, error) {
	if milestone == "" {
		return "", nil
	}
	_, parseErr := strconv.Atoi(milestone)
	if parseErr == nil {
		return milestone, nil
	}

	queryOptions := &github.MilestoneListOptions{}
	for {
		ms, res, err := issuesService.ListMilestones(ctx, repoOwner, repoName, queryOptions)
		if err != nil || res == nil || res.Response.StatusCode != http.StatusOK {
			break
		}
		for _, m := range ms {
			if m.GetTitle() == milestone {
				return strconv.Itoa(m.GetNumber()), nil
			}
		}
		queryOptions.Page++
	}

	return "", fmt.Errorf("failed to find milestone with title or number %q", milestone)
}

//counterfeiter:generate -o ./fakes_internal/issues_by_repo_lister.go --fake-name IssuesByRepoLister . issuesByRepoLister

type issuesByRepoLister interface {
	ListByRepo(ctx context.Context, owner string, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error)
}

func fetchIssuesWithLabelAndMilestone(ctx context.Context, issuesService issuesByRepoLister, repoOwner, repoName, milestoneNumber string, labels []string) ([]*github.Issue, error) {
	if milestoneNumber == "" && len(labels) == 0 {
		return nil, nil
	}
	issueList, response, err := issuesService.ListByRepo(ctx, repoOwner, repoName, &github.IssueListByRepoOptions{
		Milestone: milestoneNumber,
		Labels:    labels,
	})
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get issues: %w", err)
	}
	return issueList, nil
}

func appendFilterAndSortIssues(issuesA, issuesB []*github.Issue, filterExp string) []*github.Issue {
	fullList := insertUnique(issuesA, func(a, b *github.Issue) bool {
		return a.GetID() == b.GetID()
	}, issuesB...)

	filtered := filterIssuesByTitle(filterExp, fullList)

	sort.Sort(IssuesBySemanticTitlePrefix(filtered))

	return filtered
}

func insertUnique(list []*github.Issue, equal func(a, b *github.Issue) bool, additional ...*github.Issue) []*github.Issue {
nextNewIssue:
	for _, newIssue := range additional {
		for _, existingIssue := range list {
			if equal(newIssue, existingIssue) {
				continue nextNewIssue
			}
		}
		list = append(list, newIssue)
	}
	return list
}

func filterIssuesByTitle(exp string, issues []*github.Issue) []*github.Issue {
	if exp == "" {
		return issues
	}
	titleCheck := regexp.MustCompile(exp)
	filtered := issues[:0]
	for _, issue := range issues {
		if !titleCheck.MatchString(issue.GetTitle()) {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

// IssuesBySemanticTitlePrefix sorts issues by title lexicographically. It handles issues with a prefix like
// \*\*\[(security fix)|(feature)|(feature improvement)|(bug fix)|(none)\]\*\*, differently and sorts them
// in order of importance.
type IssuesBySemanticTitlePrefix []*github.Issue

func (list IssuesBySemanticTitlePrefix) Len() int { return len(list) }

func (list IssuesBySemanticTitlePrefix) Swap(i, j int) { list[i], list[j] = list[j], list[i] }

func (list IssuesBySemanticTitlePrefix) Less(i, j int) bool {
	it := list[i].GetTitle()
	jt := list[j].GetTitle()
	iv := issuesTitlePrefixSemanticValue(it)
	jv := issuesTitlePrefixSemanticValue(jt)
	if iv != jv {
		return iv > jv
	}
	return strings.Compare(it, jt) < 0
}

func issuesTitlePrefixSemanticValue(title string) int {
	title = strings.ToLower(title)
	prefixes := []string{
		"**[security fix]**",
		"**[feature]**",
		"**[feature improvement]**",
		"**[bug fix]**",
		"**[none]**",
	}
	for i, v := range prefixes {
		if strings.HasPrefix(title, v) {
			return len(prefixes) - i
		}
	}
	return 0
}

func (r ReleaseNotes) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "generates release notes from bosh-release release notes on GitHub between two tile repo git references",
		ShortDescription: "generates release notes from bosh-release release notes",
		Flags:            r.Options,
	}
}
