package commands

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v40/github"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/gh"
	"github.com/pivotal-cf/kiln/pkg/notes"
)

const releaseDateFormat = "2006-01-02"

type ReleaseNotes struct {
	Options struct {
		ReleaseDate  string `long:"release-date" short:"d" description:"release date of the tile"`
		TemplateName string `long:"template"     short:"t" description:"path to template"`
		GithubToken  string `long:"github-token" short:"g" description:"auth token for fetching issues merged between releases" env:"GITHUB_TOKEN"`
		GithubHost   string `long:"github-host"            description:"set this when you are using GitHub enterprise" env:"GITHUB_HOST"`
		Kilnfile     string `long:"kilnfile"     short:"k" description:"path to Kilnfile"`
		DocsFile     string `long:"update-docs"  short:"u" description:"path to docs file to update"`
		Window       string `long:"window"       short:"w" description:"GA window for release notes" default:"ga"`
		notes.IssuesQuery
		notes.TrainstatQuery
	}

	repository *git.Repository
	readFile   func(fp string) ([]byte, error)
	stat       func(name string) (fs.FileInfo, error)
	io.Writer

	fetchNotesData FetchNotesData

	repoOwner, repoName string
}

type FetchNotesData func(ctx context.Context, repo *git.Repository, client *github.Client, tileRepoOwner, tileRepoName, kilnfilePath, initialRevision, finalRevision string, issuesQuery notes.IssuesQuery, trainstatClient notes.TrainstatNotesFetcher) (notes.Data, error)

func NewReleaseNotesCommand() (ReleaseNotes, error) {
	return ReleaseNotes{
		fetchNotesData: notes.FetchData,
		readFile:       os.ReadFile,
		Writer:         os.Stdout,
		stat:           os.Stat,
	}, nil
}

func (r ReleaseNotes) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "generates release notes from bosh-release release notes on GitHub between two tile repo git references",
		ShortDescription: "generates release notes from bosh-release release notes",
		Flags:            r.Options,
	}
}

func (r ReleaseNotes) Execute(args []string) error {
	nonFlagArgs, err := jhanda.Parse(&r.Options, args)
	if err != nil {
		return err
	}

	ctx := context.Background()

	if err := r.initRepo(); err != nil {
		return err
	}

	err = r.checkInputs(nonFlagArgs)
	if err != nil {
		return err
	}

	var client *github.Client
	if r.Options.GithubToken != "" {
		client, err = gh.Client(ctx, r.Options.GithubHost, r.Options.GithubToken)
		if err != nil {
			return fmt.Errorf("failed to setup github client: %w", err)
		}
	}

	trainstatClient := notes.NewTrainstatClient(r.Options.TrainstatQuery.TrainstatURL)

	_ = notes.FetchData // fetchNotesData is github.com/pivotal/kiln/internal/notes.FetchData
	data, err := r.fetchNotesData(ctx,
		r.repository, client, r.repoOwner, r.repoName,
		r.Options.Kilnfile,
		nonFlagArgs[0], nonFlagArgs[1],
		r.Options.IssuesQuery,
		&trainstatClient,
	)
	if err != nil {
		return err
	}
	data.ReleaseDate, _ = r.parseReleaseDate()
	data.Window = r.Options.Window

	if r.Options.DocsFile == "" {
		return r.writeNotes(r.Writer, data)
	}
	return r.updateDocsFile(data)
}

func (r *ReleaseNotes) updateDocsFile(data notes.Data) error {
	// TODO: add helpful logging
	docsFileContent, err := r.readFile(r.Options.DocsFile)
	if err != nil {
		return err
	}
	page, err := notes.ParsePage(string(docsFileContent))
	if err != nil {
		return err
	}
	notes, err := data.WriteVersionNotes()
	if err != nil {
		return err
	}
	err = page.Add(notes)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	_, err = page.WriteTo(&output)
	if err != nil {
		return err
	}
	f, err := os.Create(r.Options.DocsFile)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, &output)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReleaseNotes) initRepo() error {
	if r.repository != nil {
		return nil
	}

	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	rp, err := filepath.Rel(wt.Filesystem.Root(), wd)
	if err != nil {
		return err
	}

	if rp != "." {
		return fmt.Errorf("release-notes must be run from the root of the repository (use --kilnfile flag to specify which tile to build)")
	}

	repoOwner, repoName, err := getGithubRemoteRepoOwnerAndName(repo)
	if err != nil {
		return err
	}

	r.repository = repo
	r.repoName = repoName
	r.repoOwner = repoOwner

	return nil
}

func (r ReleaseNotes) writeNotes(w io.Writer, info notes.Data) error {
	releaseNotesTemplate := notes.DefaultTemplate()
	if r.Options.TemplateName != "" {
		templateBuf, err := r.readFile(r.Options.TemplateName)
		if err != nil {
			return fmt.Errorf("failed to read provided template file: %w", err)
		}
		releaseNotesTemplate = string(templateBuf)
	}

	t, err := notes.DefaultTemplateFunctions(template.New(r.Options.TemplateName)).Parse(releaseNotesTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	err = t.Execute(w, info)
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

	if r.Options.DocsFile != "" {
		_, err := r.stat(r.Options.DocsFile)
		if err != nil {
			return err
		}
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

	repoOwner, repoName, err := gh.RepositoryOwnerAndNameFromPath(remoteURL)
	if err != nil {
		return "", "", err
	}

	return repoOwner, repoName, nil
}
