package commands

import (
	_ "embed"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/historic"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const releaseDateFormat = "2006-01-02"

type ReleaseNotes struct {
	Options struct {
		Version      string `long:"version"  short:"v"  description:"version of the tile"`      // TODO version should come from final revision not flag
		ReleaseDate  string `long:"date"     short:"rd" description:"release date of the tile"` // TODO version should come from final revision not flag
		TemplateName string `long:"template" short:"t"  description:"path to template"`
	}

	pathRelativeToDotGit string
	Repository           *git.Repository
	ReadFile             func(fp string) ([]byte, error)
	HistoricKilnfileLock
	HistoricVersion
	RevisionResolver
	Stat func(string) (os.FileInfo, error)
	io.Writer
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
	return ReleaseNotes{
		Repository:           repo,
		ReadFile:             ioutil.ReadFile,
		HistoricKilnfileLock: historic.KilnfileLock,
		HistoricVersion:      historic.Version,
		RevisionResolver:     repo,
		Stat:                 os.Stat,
		Writer:               os.Stdout,
		pathRelativeToDotGit: rp,
	}, nil
}

//counterfeiter:generate -o ./fakes/historic_version.go --fake-name HistoricVersion . HistoricVersion

type HistoricVersion func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (string, error)

//counterfeiter:generate -o ./fakes/historic_kilnfile_lock_func.go --fake-name HistoricKilnfileLock . HistoricKilnfileLock

type HistoricKilnfileLock func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (cargo.KilnfileLock, error)

//counterfeiter:generate -o ./fakes/revision_resolver.go --fake-name RevisionResolver . RevisionResolver

type RevisionResolver interface {
	ResolveRevision(rev plumbing.Revision) (*plumbing.Hash, error)
}

func (r ReleaseNotes) Execute(args []string) error {
	nonFlagArgs, err := jhanda.Parse(&r.Options, args) // TODO handle error
	if err != nil {
		return err
	}

	// TODO ensure len(nonFlagArgs) < 2

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
		// Issues:      issues,
		Components: klFinal.Releases,
		Bumps:      calculateReleaseBumps(klFinal.Releases, klInitial.Releases),
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

	return t.Execute(r.Writer, info)
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
	Version     string
	ReleaseDate time.Time
	// Issues      []*github.Issue
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
