package commands

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
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
	repository           *git.Repository
	readFile             func(fp string) ([]byte, error)
	historicKilnfileLock
	historicVersion
	revisionResolver
	stat func(string) (os.FileInfo, error)
	io.Writer

	gitHubAPIServices func(ctx context.Context, token string) githubAPIIssuesService

	repoOwner, repoName string
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
		repository:           repo,
		readFile:             ioutil.ReadFile,
		historicKilnfileLock: historic.KilnfileLock,
		historicVersion:      historic.Version,
		revisionResolver:     repo,
		stat:                 os.Stat,
		Writer:               os.Stdout,
		pathRelativeToDotGit: rp,
		repoName:             repoName,
		repoOwner:            repoOwner,

		gitHubAPIServices: func(ctx context.Context, token string) githubAPIIssuesService {
			tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
			tokenClient := oauth2.NewClient(ctx, tokenSource)
			client := github.NewClient(tokenClient)
			return client.Issues
		},
	}, nil
}

func (r ReleaseNotes) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "generates release notes from bosh-release release notes on GitHub between two tile repo git references",
		ShortDescription: "generates release notes from bosh-release release notes",
		Flags:            r.Options,
	}
}

//counterfeiter:generate -o ./fakes_internal/historic_version.go --fake-name HistoricVersion . historicVersion

type historicVersion func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (string, error)

//counterfeiter:generate -o ./fakes_internal/historic_kilnfile_lock.go --fake-name HistoricKilnfileLock . historicKilnfileLock

type historicKilnfileLock func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (cargo.KilnfileLock, error)

//counterfeiter:generate -o ./fakes_internal/revision_resolver.go --fake-name RevisionResolver . revisionResolver

type revisionResolver interface {
	ResolveRevision(rev plumbing.Revision) (*plumbing.Hash, error)
}

//go:embed release_notes.md.template
var defaultReleaseNotesTemplate string

func (r ReleaseNotes) Execute(args []string) error {
	nonFlagArgs, err := jhanda.Parse(&r.Options, args)
	if err != nil {
		return err
	}
	err = r.checkInputs(nonFlagArgs)
	if err != nil {
		return err
	}

	initialKilnfileLock, finalKilnfileLock, finalVersion, err := r.fetchHistoricFiles(nonFlagArgs[0], nonFlagArgs[1])
	if err != nil {
		return err
	}

	info := ReleaseNotesInformation{
		Version:    finalVersion,
		Components: finalKilnfileLock.Releases,
		Bumps:      calculateComponentBumps(finalKilnfileLock.Releases, initialKilnfileLock.Releases),
	}

	info.ReleaseDate, _ = r.parseReleaseDate()

	info.Issues, err = r.listGithubIssues(context.TODO())
	if err != nil {
		return err
	}

	err = r.encodeReleaseNotes(info)
	if err != nil {
		return err
	}

	return nil
}

func (r ReleaseNotes) fetchHistoricFiles(start, end string) (cargo.KilnfileLock, cargo.KilnfileLock, *semver.Version, error) {
	initialCommitSHA, err := r.ResolveRevision(plumbing.Revision(start))
	if err != nil {
		return cargo.KilnfileLock{}, cargo.KilnfileLock{}, nil, fmt.Errorf("failed to resolve inital revision %q: %w", start, err)
	}
	finalCommitSHA, err := r.ResolveRevision(plumbing.Revision(end))
	if err != nil {
		return cargo.KilnfileLock{}, cargo.KilnfileLock{}, nil, fmt.Errorf("failed to resolve final revision %q: %w", end, err)
	}

	klInitial, err := r.historicKilnfileLock(r.repository, *initialCommitSHA, r.pathRelativeToDotGit)
	if err != nil {
		return cargo.KilnfileLock{}, cargo.KilnfileLock{}, nil, fmt.Errorf("failed to get kilnfile from initial commit: %w", err)
	}
	klFinal, err := r.historicKilnfileLock(r.repository, *finalCommitSHA, r.pathRelativeToDotGit)
	if err != nil {
		return cargo.KilnfileLock{}, cargo.KilnfileLock{}, nil, fmt.Errorf("failed to get kilnfile from final commit: %w", err)
	}
	version, err := r.historicVersion(r.repository, *finalCommitSHA, r.pathRelativeToDotGit)
	if err != nil {
		return cargo.KilnfileLock{}, cargo.KilnfileLock{}, nil, fmt.Errorf("failed to get version file from final commit: %w", err)
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return cargo.KilnfileLock{}, cargo.KilnfileLock{}, nil, fmt.Errorf("failed to parse version: %w", err)
	}

	return klInitial, klFinal, v, nil
}

func (r ReleaseNotes) encodeReleaseNotes(info ReleaseNotesInformation) error {
	releaseNotesTemplate := defaultReleaseNotesTemplate
	if r.Options.TemplateName != "" {
		templateBuf, err := r.readFile(r.Options.TemplateName)
		if err != nil {
			return fmt.Errorf("failed to read provided template file: %w", err)
		}
		releaseNotesTemplate = string(templateBuf)
	}

	t, err := template.New(r.Options.TemplateName).Parse(releaseNotesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	err = t.Execute(r.Writer, info)
	if err != nil {
		return err
	}

	return nil
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
		(r.Options.IssueMilestone != "" ||
			len(r.Options.IssueIDs) > 0 ||
			len(r.Options.IssueLabels) > 0) {
		return errors.New("github-token (env: GITHUB_TOKEN) must be set to interact with the github api")
	}

	_, err := r.parseReleaseDate()
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

type ReleaseNotesInformation struct {
	Version     *semver.Version
	ReleaseDate time.Time

	Issues []*github.Issue

	Bumps      []BoshReleaseBump
	Components []component.Lock
}

//counterfeiter:generate -o ./fakes_internal/release_notes_github_api_issues_service.go --fake-name GithubAPIIssuesService . githubAPIIssuesService

type githubAPIIssuesService interface {
	issueGetter
	milestoneLister
	issuesByRepoLister
}

// listGithubIssues is not tested. By not testing we are getting reduced abstraction and improved readability at the
// cost of high level testing. This function therefore must stay as small as possible and rely on type checking and a
// manual test to ensure it continues to behave as expected during refactors.
//
// The function can be tested by generating release notes for a tile with issue ids and a milestone set. The happy path
// test for Execute does not set GithubToken intentionally so this code is not triggered and Execute does not actually
// reach out to GitHub.
func (r ReleaseNotes) listGithubIssues(ctx context.Context) ([]*github.Issue, error) {
	if r.Options.GithubToken == "" {
		return nil, nil
	}

	issuesService := r.gitHubAPIServices(ctx, r.Options.GithubToken)

	milestoneNumber, err := resolveMilestoneNumber(ctx, issuesService, r.repoOwner, r.repoName, r.Options.IssueMilestone)
	if err != nil {
		return nil, err
	}
	issues, err := fetchIssuesWithLabelAndMilestone(ctx, issuesService, r.repoOwner, r.repoName, milestoneNumber, r.Options.IssueLabels)
	if err != nil {
		return nil, err
	}
	additionalIssues, err := issuesFromIssueIDs(ctx, issuesService, r.repoOwner, r.repoName, r.Options.IssueIDs)
	if err != nil {
		return nil, err
	}

	return appendFilterAndSortIssues(issues, additionalIssues, r.Options.IssueTitleExp), nil
}
