package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v2"
)

type Glaze struct {
	Options struct {
		Kilnfile string `short:"kf" long:"kilnfile" default:"Kilnfile"  description:"path to Kilnfile"`
	}
}

func (cmd *Glaze) Execute(args []string) error {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return err
	}

	if info, err := os.Stat(cmd.Options.Kilnfile); err == nil && info.IsDir() {
		cmd.Options.Kilnfile = filepath.Join(cmd.Options.Kilnfile, "Kilnfile")
	}

	kf, err := os.ReadFile(cmd.Options.Kilnfile)
	if err != nil {
		return err
	}

	var kilnfile cargo.Kilnfile
	err = yaml.Unmarshal(kf, &kilnfile)
	if err != nil {
		return fmt.Errorf("failed to load Kilnfile: %w", err)
	}

	kfl, err := os.ReadFile(cmd.Options.Kilnfile + ".lock")
	if err != nil {
		return err
	}

	var kilnfileLock cargo.KilnfileLock
	err = yaml.Unmarshal(kfl, &kilnfileLock)
	if err != nil {
		return fmt.Errorf("failed to load Kilnfile.lock: %w", err)
	}

	kilnfile.Stemcell.Version = kilnfileLock.Stemcell.Version
	for releaseIndex, release := range kilnfile.Releases {
		l, err := kilnfileLock.FindReleaseWithName(release.Name)
		if err != nil {
			return fmt.Errorf("release with name %q not found in Kilnfile.lock: %w", release.Name, err)
		}

		kilnfile.Releases[releaseIndex].Version = l.Version
	}

	kf, err = yaml.Marshal(kilnfile)
	if err != nil {
		return err
	}

	outputKilnfile, err := os.Create(cmd.Options.Kilnfile)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(outputKilnfile)
	_, err = outputKilnfile.Write(kf)
	return err
}

func (cmd *Glaze) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command locks all the components.",
		ShortDescription: "Pin versions in Kilnfile to match lock.",
	}
}
