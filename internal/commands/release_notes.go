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
	Stat func(string) (os.FileInfo, error)
	io.Writer

	RemoteURL, RepoOwner, RepoName string
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

	repoURL, repoOwner, repoName, err := getGithubRemoteRepoOwnerAndName(repo)
	if err != nil {
		return ReleaseNotes{}, err
	}

	return ReleaseNotes{
		Repository:           repo,
		ReadFile:             ioutil.ReadFile,
		HistoricKilnfileLock: historic.KilnfileLock,
		HistoricVersion:      historic.Version,
		RevisionResolver:     repo,
		Stat:                 os.Stat,
		Writer:               os.Stdout,
		pathRelativeToDotGit: rp,
		RepoName:             repoName,
		RepoOwner:            repoOwner,
		RemoteURL:            repoURL,
	}, nil
}

//counterfeiter:generate -o ./fakes/historic_kilnfile_lock.go --fake-name HistoricKilnfileLock . HistoricKilnfileLock

type HistoricKilnfileLock func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (cargo.KilnfileLock, error)

//counterfeiter:generate -o ./fakes/historic_version.go --fake-name HistoricVersion . HistoricVersion

type HistoricVersion func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (string, error)

//counterfeiter:generate -o ./fakes/revision_resolver.go --fake-name RevisionResolver . RevisionResolver

type RevisionResolver interface {
	ResolveRevision(rev plumbing.Revision) (*plumbing.Hash, error)
}

func (r ReleaseNotes) Execute(args []string) error {
	nonFlagArgs, err := jhanda.Parse(&r.Options, args) // TODO handle error
	if err != nil {
		return err
	}
	err = r.checkInputs(nonFlagArgs)
	if err != nil {
		return err
	}

	releaseDate, err := r.parseReleaseDate()
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
		Version:           v, // TODO version should come from version file at final revision and then maybe override with flag
		ReleaseDate:       releaseDate,
		ReleaseDateFormat: releaseDateFormat,
		Components:        klFinal.Releases,
		Bumps:             calculateReleaseBumps(klFinal.Releases, klInitial.Releases),
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

//go:embed release_notes.md.template
var defaultReleaseNotesTemplate string

type ReleaseNotesInformation struct {
	Version           *semver.Version
	ReleaseDate       time.Time
	ReleaseDateFormat string

	Issues []*github.Issue

	Bumps      []component.Lock
	Components []component.Lock
}

type BoshReleaseBump = component.Lock

func calculateReleaseBumps(current, previous []component.Lock) []BoshReleaseBump {
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

func (r ReleaseNotes) listGithubIssues(ctx context.Context) ([]*github.Issue, error) {
	if r.Options.GithubToken == "" {
		return nil, nil
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: r.Options.GithubToken})
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(tokenClient)

	issues, err := issuesFromIssueIDs(ctx, githubClient, r.RepoOwner, r.RepoName, r.Options.IssueIDs)
	if err != nil {
		return nil, err
	}
	milestoneNumber, err := resolveMilestoneNumber(ctx, githubClient, r.RepoOwner, r.RepoName, r.Options.IssueMilestone)
	if err != nil {
		return nil, err
	}
	milestoneLabeledIssues, err := fetchIssuesWithLabelAndMilestone(ctx, githubClient, r.RepoOwner, r.RepoName, milestoneNumber, r.Options.IssueLabels)
	if err != nil {
		return nil, err
	}
	return appendFilterAndSortIssues(issues, milestoneLabeledIssues, r.Options.IssueTitleExp), nil
}

func appendFilterAndSortIssues(issuesA, issuesB []*github.Issue, filterExp string) []*github.Issue {
	fullList := insertUnique(issuesA, func(a, b *github.Issue) bool {
		return a.GetID() == b.GetID()
	}, issuesB...)

	filtered := filterIssuesByTitle(filterExp, fullList)

	sort.Sort(IssuesBySemanticTitlePrefix(filtered))

	return filtered
}

func fetchIssuesWithLabelAndMilestone(ctx context.Context, githubClient *github.Client, repoOwner, repoName, milestoneNumber string, labels []string) ([]*github.Issue, error) {
	if milestoneNumber == "" && len(labels) == 0 {
		return nil, nil
	}
	issueList, response, err := githubClient.Issues.ListByRepo(ctx, repoOwner, repoName, &github.IssueListByRepoOptions{
		Milestone: milestoneNumber,
		Labels:    labels,
	})
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get issues %q: %w", response.Request.URL, err)
	}
	return issueList, nil
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

func resolveMilestoneNumber(ctx context.Context, githubClient *github.Client, repoOwner, repoName, milestone string) (string, error) {
	if milestone == "" {
		return "", nil
	}
	_, parseErr := strconv.Atoi(milestone)
	if parseErr == nil {
		return milestone, nil
	}

	queryOptions := &github.MilestoneListOptions{}
	for {
		ms, _, err := githubClient.Issues.ListMilestones(ctx, repoOwner, repoName, queryOptions)
		if err != nil {
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

func issuesFromIssueIDs(ctx context.Context, githubClient *github.Client, repoOwner, repoName string, issueIDs []string) ([]*github.Issue, error) {
	var issues []*github.Issue
	for _, id := range issueIDs {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("failed to parse issue id %q: %w", id, err)
		}
		issue, response, err := githubClient.Issues.Get(ctx, repoOwner, repoName, n)
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
		(r.Options.IssueTitleExp != "" || len(r.Options.IssueIDs) > 0 || len(r.Options.IssueLabels) > 0) {
		return errors.New("github-token (env: GITHUB_TOKEN) must be set to interact with the github api")
	}

	return nil
}

func getGithubRemoteRepoOwnerAndName(repo *git.Repository) (string, string, string, error) {
	remotes, err := repo.Remotes()
	if err != nil {
		return "", "", "", err
	}

	var remoteURL string
	for _, remote := range remotes {
		config := remote.Config()
		if config == nil {
			continue
		}
		for _, u := range config.URLs {
			if !strings.Contains(u, "github.com") {
				continue
			}
			remoteURL = u
			break
		}
	}
	if remoteURL == "" {
		return "", "", "", fmt.Errorf("remote github URL not found for repo")
	}

	repoOwner, repoName := component.OwnerAndRepoFromGitHubURI(remoteURL)
	if repoOwner == "" || repoName == "" {
		return "", "", "", errors.New("could not determine owner and repo for tile")
	}

	return fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName),
		repoOwner, repoName, nil
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
