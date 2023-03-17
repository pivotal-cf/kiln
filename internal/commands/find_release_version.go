package commands

import (
	"context"
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

type releaseVersionOutput struct {
	Version    string `json:"version"`
	RemotePath string `json:"remote_path"`
	Source     string `json:"source"`
	SHA        string `json:"sha"`
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
	releaseSource := cmd.mrsProvider(kilnfile, false)

	spec, ok := kilnfile.ComponentSpec(cmd.Options.Release)
	if !ok {
		return cargo.ErrorSpecNotFound(cmd.Options.Release)
	}

	spec.StemcellOS = kilnfileLock.Stemcell.OS
	spec.StemcellVersion = kilnfileLock.Stemcell.Version

	releaseRemote, err := releaseSource.FindReleaseVersion(context.Background(), spec, cmd.Options.NoDownload)
	if err != nil {
		return err
	}

	releaseVersionJSON, _ := json.Marshal(releaseVersionOutput{
		Version:    releaseRemote.Version,
		RemotePath: releaseRemote.RemotePath,
		Source:     releaseRemote.RemoteSource,
		SHA:        releaseRemote.SHA1,
	})
	cmd.outLogger.Println(string(releaseVersionJSON))
	return err
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
