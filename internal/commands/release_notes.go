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
	"text/template"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v50/github"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/gh"
	"github.com/pivotal-cf/kiln/pkg/notes"
)

const releaseDateFormat = "2006-01-02"

type ReleaseNotes struct {
	Options struct {
		ReleaseDate                 string   `long:"release-date"   short:"d"  description:"release date of the tile"`
		TemplateName                string   `long:"template"       short:"t"  description:"path to template"`
		GithubAccessToken           string   `long:"github_access_token"   short:"g"  description:"auth token for github.com" env:"GITHUB_ACCESS_TOKEN"`
		GithubEnterpriseAccessToken string   `long:"github_enterprise_access_token"  short:"ge"   description:"auth token for github enterprise" env:"GITHUB_ENTERPRISE_ACCESS_TOKEN"`
		Kilnfile                    string   `long:"kilnfile"       short:"k"  description:"path to Kilnfile"`
		DocsFile                    string   `long:"update-docs"    short:"u"  description:"path to docs file to update"`
		Window                      string   `long:"window"         short:"w"  description:"GA window for release notes" default:"ga"`
		VariableFiles               []string `long:"variables-file" short:"vf" description:"path to a file containing variables to interpolate"`
		Variables                   []string `long:"variable"       short:"vr" description:"key value pairs of variables to interpolate"`
		notes.IssuesQuery
		notes.TrainstatQuery
	}

	repository *git.Repository
	readFile   func(fp string) ([]byte, error)
	stat       func(name string) (fs.FileInfo, error)
	io.Writer

	fetchNotesData   FetchNotesData
	variablesService baking.TemplateVariablesService

	repoHost, repoOwner, repoName string
}

type FetchNotesData func(ctx context.Context, repo *git.Repository, client *github.Client, tileRepoHost, tileRepoOwner, tileRepoName, kilnfilePath, initialRevision, finalRevision string, issuesQuery notes.IssuesQuery, trainstatClient notes.TrainstatNotesFetcher, variables map[string]any) (notes.Data, error)

func NewReleaseNotesCommand() (ReleaseNotes, error) {
	return ReleaseNotes{
		variablesService: baking.NewTemplateVariablesService(osfs.New(".")),
		fetchNotesData:   notes.FetchData,
		readFile:         os.ReadFile,
		Writer:           os.Stdout,
		stat:             os.Stat,
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

	templateVariables, err := r.variablesService.FromPathsAndPairs(r.Options.VariableFiles, r.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}
	if varValue, ok := templateVariables["github_access_token"]; !ok && r.Options.GithubAccessToken != "" {
		templateVariables["github_access_token"] = r.Options.GithubAccessToken
	} else if ok && r.Options.GithubAccessToken == "" {
		r.Options.GithubAccessToken = varValue.(string)
	}

	if varValue, ok := templateVariables["github_enterprise_access_token"]; !ok && r.Options.GithubAccessToken != "" {
		templateVariables["github_enterprise_access_token"] = r.Options.GithubEnterpriseAccessToken
	} else if ok && r.Options.GithubEnterpriseAccessToken == "" {
		r.Options.GithubEnterpriseAccessToken = varValue.(string)
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

	client, err = gh.Client(ctx, r.repoHost, r.Options.GithubAccessToken, r.Options.GithubEnterpriseAccessToken)
	if err != nil {
		return fmt.Errorf("failed to setup github client: %w", err)
	}

	trainstatClient := notes.NewTrainstatClient(r.Options.TrainstatQuery.TrainstatURL)

	_ = notes.FetchData // fetchNotesData is github.com/pivotal/kiln/internal/notes.FetchData
	data, err := r.fetchNotesData(ctx,
		r.repository, client, r.repoHost, r.repoOwner, r.repoName,
		r.Options.Kilnfile,
		nonFlagArgs[0], nonFlagArgs[1],
		r.Options.IssuesQuery,
		&trainstatClient,
		templateVariables,
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

	repoHost, repoOwner, repoName, err := getGithubRemoteHostRepoOwnerAndName(repo)
	if err != nil {
		return err
	}

	r.repository = repo
	r.repoHost = repoHost
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

	if r.Options.GithubEnterpriseAccessToken == "" && r.Options.GithubAccessToken == "" {
		return errors.New("github_access_token(env: GITHUB_ACCESS_TOKEN) and/or github_enterprise_access_token(env: GITHUB_ENTERPRISE_ACCESS_TOKEN) must be set to interact with the github api")
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

func getGithubRemoteHostRepoOwnerAndName(repo *git.Repository) (string, string, string, error) {
	var remoteURL string
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", "", err
	}
	config := remote.Config()
	for _, u := range config.URLs {
		remoteURL = u
		break
	}
	if remoteURL == "" {
		return "", "", "", fmt.Errorf("remote github URL not found for repo")
	}

	repoHost, repoOwner, repoName, err := gh.RepositoryHostOwnerAndNameFromPath(remoteURL)
	if err != nil {
		return "", "", "", err
	}

	return repoHost, repoOwner, repoName, nil
}
