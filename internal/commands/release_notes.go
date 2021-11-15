package commands

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"golang.org/x/oauth2"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v40/github"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/historic"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const releaseDateFormat = "2006-01-02"

type ReleaseNotes struct {
	Options struct {
		ReleaseDate  string   `required:"true" long:"date"         short:"rd" description:"release date of the tile"`
		TemplateName string   `                long:"template"     short:"t"  description:"path to template"`
		GithubToken  string   `                long:"github-token" short:"g"  description:"auth token for fetching issues merged between releases" env:"GITHUB_TOKEN"`
		IssueIDs     []string `             long:"github-issue" short:"i"  description:"a list of issues to include in the release notes"`
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

//counterfeiter:generate -o ./fakes/historic_version.go --fake-name HistoricVersion . HistoricVersion

type HistoricVersion func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (string, error)

//counterfeiter:generate -o ./fakes/historic_kilnfile_lock_func.go --fake-name HistoricKilnfileLock . HistoricKilnfileLock

type HistoricKilnfileLock func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (cargo.KilnfileLock, error)

//counterfeiter:generate -o ./fakes/historic_version.go --fake-name HistoricVersionFunc . HistoricVersionFunc

type HistoricVersionFunc func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (string, error)

//counterfeiter:generate -o ./fakes/get_github_issue.go --fake-name GetGithubIssue . GetGithubIssue

type GetIssueFunc func(ctx context.Context, owner string, repo string, number int) (*github.Issue, *github.Response, error)

//counterfeiter:generate -o ./fakes/revision_resolver.go --fake-name RevisionResolver . RevisionResolver

type RevisionResolver interface {
	ResolveRevision(rev plumbing.Revision) (*plumbing.Hash, error)
}

func (r ReleaseNotes) Execute(args []string) error {
	nonFlagArgs, err := jhanda.Parse(&r.Options, args) // TODO handle error
	if err != nil {
		return err
	}

	if len(nonFlagArgs) != 2 {
		return errors.New("expected two arguments: <Git-Revision> <Git-Revision>")
	}

	releaseDate, err := time.Parse(releaseDateFormat, r.Options.ReleaseDate)
	if err != nil {
		return fmt.Errorf("release date could not be parsed: %w", err)
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

	info := ReleaseNotesInformation{
		Version:     version,
		ReleaseDate: releaseDate,
		Components:  klFinal.Releases,
		Bumps:       calculateReleaseBumps(klFinal.Releases, klInitial.Releases),
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

func (r ReleaseNotes) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "generates release notes from bosh-release release notes on GitHub between two tile repo git references",
		ShortDescription: "generates release notes from bosh-release release notes",
		Flags:            r.Options,
	}
}

//go:embed release_notes.md.template
var defaultReleaseNotesTemplate string

type ReleaseNotesInformation struct {
	Version           string
	ReleaseDate       time.Time
	ReleaseDateFormat string

	Issues []*github.Issue

	Bumps      []component.Lock
	Components []component.Lock
}

type BoshReleaseBump = component.Spec

func calculateReleaseBumps(current, previous []component.Lock) []component.Lock {
	var (
		bumps         []component.Lock
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
	if r.Options.GithubToken == "" || len(r.Options.IssueIDs) == 0 {
		return nil, nil
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: r.Options.GithubToken})
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(tokenClient)

	var issues []*github.Issue
	for _, id := range r.Options.IssueIDs {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("failed to parse issue id %q: %w", id, err)
		}
		issue, response, err := githubClient.Issues.Get(ctx, r.RepoOwner, r.RepoName, n)
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
