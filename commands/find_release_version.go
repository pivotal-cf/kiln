package commands

import (
	"encoding/json"
	cargo2 "github.com/pivotal-cf/kiln/pkg/cargo"
	release2 "github.com/pivotal-cf/kiln/pkg/release"
	"log"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4/osfs"
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
	Source     string `json:"source"`
	SHA        string `json:"sha"`
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

	releaseRemote, _, err := releaseSource.FindReleaseVersion(release2.Requirement{
		Name:              cmd.Options.Release,
		VersionConstraint: version,
		StemcellVersion:   kilnfileLock.Stemcell.Version,
		StemcellOS:        kilnfileLock.Stemcell.OS,
	})

	releaseVersionJson, _ := json.Marshal(releaseVersionOutput{
		Version:    releaseRemote.Version,
		RemotePath: releaseRemote.RemotePath,
		SHA:        releaseRemote.SHA,
		Source:     releaseRemote.SourceID,
	})
	cmd.outLogger.Println(string(releaseVersionJson))
	return err
}

func (cmd *FindReleaseVersion) setup(args []string) (cargo2.Kilnfile, cargo2.KilnfileLock, error) {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return cargo2.Kilnfile{}, cargo2.KilnfileLock{}, err
	}

	kilnfile, kilnfileLock, err := cargo2.KilnfileLoader{}.LoadKilnfiles(osfs.New(""), cmd.Options.Kilnfile, cmd.Options.VariablesFiles, cmd.Options.Variables)
	if err != nil {
		return cargo2.Kilnfile{}, cargo2.KilnfileLock{}, err
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
