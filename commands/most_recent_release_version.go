package commands

import (
	"encoding/json"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"log"
)

type MostRecentReleaseVersion struct {
	outLogger   *log.Logger
	mrsProvider MultiReleaseSourceProvider

	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		Release        string   `short:"r" long:"release" default:"releases" description:"release name"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
	}
}

type mostRecentVersionOutput struct {
	Version    string `json:"version"`
	RemotePath string `json:"remote_path"`
}

func NewMostRecentReleaseVersion(outLogger *log.Logger, multiReleaseSourceProvider MultiReleaseSourceProvider) MostRecentReleaseVersion {
	return MostRecentReleaseVersion{
		outLogger:   outLogger,
		mrsProvider: multiReleaseSourceProvider,
	}
}

func (cmd MostRecentReleaseVersion) Execute(args []string) error {
	kilnfile, err := cmd.setup(args)
	if err != nil {
		return err
	}
	releaseSource := cmd.mrsProvider(kilnfile, false)

	releaseRemote, _, err := releaseSource.GetLatestReleaseVersion(release.Requirement{
		Name: cmd.Options.Release,
	})

	mostRecentVersionJson, _ := json.Marshal(mostRecentVersionOutput{
		Version:    releaseRemote.Version,
		RemotePath: releaseRemote.RemotePath,
	})
	cmd.outLogger.Println(string(mostRecentVersionJson))
	return err
}

func (cmd *MostRecentReleaseVersion) setup(args []string) (cargo.Kilnfile, error) {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return cargo.Kilnfile{}, err
	}

	kilnfile, _, err := cargo.KilnfileLoader{}.LoadKilnfiles(osfs.New(""), cmd.Options.Kilnfile, cmd.Options.VariablesFiles, cmd.Options.Variables)
	if err != nil {
		return cargo.Kilnfile{}, err
	}

	return kilnfile, nil
}

func (cmd MostRecentReleaseVersion) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Replace later",
		ShortDescription: "rplce ltr",
		Flags:            cmd.Options,
	}
}
