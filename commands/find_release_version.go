package commands

import (
	"encoding/json"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"log"
)

type FindReleaseVersion struct {
	outLogger   *log.Logger
	mrsProvider MultiReleaseSourceProvider

	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		Release        string   `short:"r" long:"release" default:"releases" description:"release name"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
	}
}

type releaseVersionOutput struct {
	Version    string `json:"version"`
	RemotePath string `json:"remote_path"`
}

func NewFindReleaseVersion(outLogger *log.Logger, multiReleaseSourceProvider MultiReleaseSourceProvider) FindReleaseVersion {
	return FindReleaseVersion{
		outLogger:   outLogger,
		mrsProvider: multiReleaseSourceProvider,
	}
}

func (cmd FindReleaseVersion) Execute(args []string) error {
	kilnfile, kilnfileLock, err := cmd.setup(args)
	if err != nil {
		return err
	}
	releaseSource := cmd.mrsProvider(kilnfile, false)

	var version string
	for _, release := range kilnfile.Releases {
		if release.Name == cmd.Options.Release {
			version = release.Version
			break
		}
	}

	releaseRemote, _, err := releaseSource.FindReleaseVersion(release.Requirement{
		Name: cmd.Options.Release,
		Version: version,
		StemcellVersion: kilnfileLock.Stemcell.Version,
		StemcellOS:  kilnfileLock.Stemcell.OS,
	})

	releaseVersionJson, _ := json.Marshal(releaseVersionOutput{
		Version:    releaseRemote.Version,
		RemotePath: releaseRemote.RemotePath,
	})
	cmd.outLogger.Println(string(releaseVersionJson))
	return err
}

func (cmd *FindReleaseVersion) setup(args []string) (cargo.Kilnfile, cargo.KilnfileLock, error) {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	kilnfile, kilnfileLock, err := cargo.KilnfileLoader{}.LoadKilnfiles(osfs.New(""), cmd.Options.Kilnfile, cmd.Options.VariablesFiles, cmd.Options.Variables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	return kilnfile, kilnfileLock, nil
}

func (cmd FindReleaseVersion) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Prints a json string of a remote release satisfying the Kilnfile version and stemcell constraints.",
		ShortDescription: "prints a json string of a remote release satisfying the Kilnfile version and stemcell constraints",
		Flags:            cmd.Options,
	}
}
