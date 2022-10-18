package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type FindReleaseVersion struct {
	outLogger   *log.Logger
	mrsProvider MultiReleaseSourceProvider

	Options struct {
		flags.Standard
		Release    string `short:"r" long:"release" description:"release name"`
		NoDownload bool   `long:"no-download" description:"do not download any files"`
	}
}

func NewFindReleaseVersion(outLogger *log.Logger, multiReleaseSourceProvider MultiReleaseSourceProvider) *FindReleaseVersion {
	return &FindReleaseVersion{
		outLogger:   outLogger,
		mrsProvider: multiReleaseSourceProvider,
	}
}

func (cmd *FindReleaseVersion) Execute(args []string) error {
	kilnfile, kilnfileLock, err := cmd.setup(args)
	if err != nil {
		return err
	}

	spec, ok := kilnfile.ComponentSpec(cmd.Options.Release)
	if !ok {
		return cargo.ErrorSpecNotFound(cmd.Options.Release)
	}

	releaseSources := cmd.mrsProvider(kilnfile, false)
	releaseSource, err := releaseSources.FindByID(spec.ReleaseSource)
	if err != nil {
		return errReleaseSourceNotSpecified(cmd.Options.Release)
	}

	spec.StemcellOS = kilnfileLock.Stemcell.OS
	spec.StemcellVersion = kilnfileLock.Stemcell.Version

	releaseRemote, err := releaseSource.FindReleaseVersion(spec, cmd.Options.NoDownload)
	if err != nil {
		return err
	}

	releaseVersionJson, _ := json.Marshal(struct {
		Version    string `json:"version"`
		RemotePath string `json:"remote_path"`
		Source     string `json:"source"`
		SHA        string `json:"sha"`
	}{
		Version:    releaseRemote.Version,
		RemotePath: releaseRemote.RemotePath,
		Source:     releaseRemote.RemoteSource,
		SHA:        releaseRemote.SHA1,
	})
	cmd.outLogger.Println(string(releaseVersionJson))
	return err
}

func errReleaseSourceNotSpecified(releaseName string) error {
	return fmt.Errorf("release source not specified in Kilnfile for %q", releaseName)
}

func (cmd *FindReleaseVersion) setup(args []string) (cargo.Kilnfile, cargo.KilnfileLock, error) {
	argsAfterFlags, err := flags.LoadFlagsWithDefaults(&cmd.Options, args, nil)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	if cmd.Options.Release == "" {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, errors.New("missing required flag \"--release\"")
	}

	if len(argsAfterFlags) != 0 {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, fmt.Errorf("unexpected arguments: %v", argsAfterFlags)
	}

	kilnfile, kilnfileLock, err := cmd.Options.LoadKilnfiles(nil, nil)
	if err != nil {
		fmt.Println(err)
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	return kilnfile, kilnfileLock, nil
}

func (cmd *FindReleaseVersion) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Prints a json string of a remote release satisfying the Kilnfile version and stemcell constraints.",
		ShortDescription: "prints a json string of a remote release satisfying the Kilnfile version and stemcell constraints",
		Flags:            cmd.Options,
	}
}
