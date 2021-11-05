package commands

import (
	_ "embed"
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
	"github.com/pivotal-cf/kiln/internal/history"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type ReleaseNotes struct {
	Options struct {
		Version      string `short:"v" long:"version" description:"version of the tile"` // TODO version should come from final revision not flag
		ReleaseDate  time.Time `short:"rd" long:"date" description:"release date of the tile"` // TODO version should come from final revision not flag
		TemplateName string `short:"t" long:"template" description:"path to template"`
	}

	pathRelativeToDotGit string
	Repository           *git.Repository
	ReadFile             func(fp string) ([]byte, error)
	KilnfileLockAtCommit HistoricKilnfileLockFunc
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
		KilnfileLockAtCommit: history.KilnfileLock,
		RevisionResolver:     repo,
		Stat:                 os.Stat,
		Writer:               os.Stdout,
		pathRelativeToDotGit: rp,
	}, nil
}

//counterfeiter:generate -o ./fakes/historic_kilnfile_lock_func.go --fake-name HistoricKilnfileLockFunc . HistoricKilnfileLockFunc

type HistoricKilnfileLockFunc func(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (cargo.KilnfileLock, error)

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

	initialCommitSHA, err := r.ResolveRevision(plumbing.Revision(nonFlagArgs[0])) // TODO handle error
	if err != nil {
		panic(err)
	}
	finalCommitSHA, err := r.ResolveRevision(plumbing.Revision(nonFlagArgs[1]))   // TODO handle error
	if err != nil {
		panic(err)
	}

	klInitial, err := r.KilnfileLockAtCommit(r.Repository, *initialCommitSHA, r.pathRelativeToDotGit) // TODO handle error
	if err != nil {
		panic(err)
	}
	klFinal, err := r.KilnfileLockAtCommit(r.Repository, *finalCommitSHA, r.pathRelativeToDotGit)     // TODO handle error
	if err != nil {
		panic(err)
	}

	info := newReleaseInfoInformation(
		r.Options.Version, // TODO version should come from version file at final revision and then maybe override with flag
		r.Options.ReleaseDate,
		klFinal.Releases,
		klInitial.Releases,
	)

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

func newReleaseInfoInformation(v string, rd time.Time, current, previous []component.Lock /* issues []*github.Issue*/) ReleaseNotesInformation {
	info := ReleaseNotesInformation{
		Version:     v,
		ReleaseDate: rd,
		// Issues:      issues,
		Components: current,
	}

	previousSpecs := make(map[component.Spec]struct{}, len(previous))
	for _, cs := range previous {
		previousSpecs[cs.ComponentSpec] = struct{}{}
	}
	for _, cs := range current {
		_, isSame := previousSpecs[cs.ComponentSpec]
		if isSame {
			continue
		}
		info.Bumps = append(info.Bumps, cs)
	}

	return info
}
