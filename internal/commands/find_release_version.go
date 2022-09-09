package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/internal/component"
	"log"

	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type FindReleaseVersion struct {
	outLogger *log.Logger
	Finder    component.ReleaseVersionFinder

	Options struct {
		flags.Standard
		Release string `short:"r" long:"release" description:"release name"`
	}
}

type releaseVersionOutput struct {
	Version    string `json:"version"`
	RemotePath string `json:"remote_path"`
	Source     string `json:"source"`
	SHA        string `json:"sha"`
}

func NewFindReleaseVersion(outLogger *log.Logger) *FindReleaseVersion {
	return &FindReleaseVersion{
		outLogger: outLogger,
	}
}

func (cmd *FindReleaseVersion) Execute(args []string) error {
	ctx := context.Background()
	kilnfile, kilnfileLock, err := cmd.setup(args)
	if err != nil {
		return err
	}

	if cmd.Finder == nil {
		cmd.Finder = kilnfile.ReleaseSources
	}

	spec, err := kilnfile.FindReleaseWithName(cmd.Options.Release)
	if err != nil {
		return err
	}

	spec.StemcellOS = kilnfileLock.Stemcell.OS
	spec.StemcellVersion = kilnfileLock.Stemcell.Version

	releaseRemote, err := cmd.Finder.FindReleaseVersion(ctx, cmd.outLogger, spec)
	if err != nil {
		return err
	}

	releaseVersionJson, _ := json.Marshal(releaseVersionOutput{
		Version:    releaseRemote.Version,
		RemotePath: releaseRemote.RemotePath,
		Source:     releaseRemote.RemoteSource,
		SHA:        releaseRemote.SHA1,
	})
	cmd.outLogger.Println(string(releaseVersionJson))
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
