package commands

import (
	"encoding/json"
	"log"

	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/options"
	"github.com/pivotal-cf/kiln/internal/component"
)

type FindReleaseVersion struct {
	outLogger   *log.Logger
	mrsProvider MultiReleaseSourceProvider

	Options struct {
		options.Standard
		Release string `short:"r" long:"release" default:"releases" description:"release name"`
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

func (f FindReleaseVersion) Execute(args []string) error {
	return Kiln{
		Wrapped:       f,
		KilnfileStore: KilnfileStore{},
	}.Execute(args)
}

func (f FindReleaseVersion) KilnExecute(args []string, parseOpts OptionsParseFunc) error {
	kilnfile, kilnfileLock, _, err := parseOpts(args, &f.Options)
	if err != nil {
		return err
	}

	releaseSource := f.mrsProvider(kilnfile, false)

	var version string
	for _, r := range kilnfile.Releases {
		if r.Name == f.Options.Release {
			version = r.Version
			break
		}
	}

	releaseRemote, _, err := releaseSource.FindReleaseVersion(component.Spec{
		Name:            f.Options.Release,
		Version:         version,
		StemcellVersion: kilnfileLock.Stemcell.Version,
		StemcellOS:      kilnfileLock.Stemcell.OS,
	})

	releaseVersionJson, _ := json.Marshal(releaseVersionOutput{
		Version:    releaseRemote.Version,
		RemotePath: releaseRemote.RemotePath,
		Source:     releaseRemote.RemoteSource,
		SHA:        releaseRemote.SHA1,
	})
	f.outLogger.Println(string(releaseVersionJson))
	return err
}

func (f FindReleaseVersion) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Prints a json string of a remote release satisfying the Kilnfile version and stemcell constraints.",
		ShortDescription: "prints a json string of a remote release satisfying the Kilnfile version and stemcell constraints",
		Flags:            f.Options,
	}
}
