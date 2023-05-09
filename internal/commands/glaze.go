package commands

import (
	"fmt"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/pkg/cargo"
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

	kfPath, err := cargo.ResolveKilnfilePath(cmd.Options.Kilnfile)
	if err != nil {
		return err
	}

	kilnfile, kilnfileLock, err := cargo.ReadKilnfiles(kfPath)
	if err != nil {
		return err
	}

	kilnfile, err = pinVersions(kilnfile, kilnfileLock)
	if err != nil {
		return err
	}

	return cargo.WriteKilnfile(kfPath, kilnfile)
}

func (cmd *Glaze) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command locks all the components.",
		ShortDescription: "Pin versions in Kilnfile to match lock.",
	}
}

func pinVersions(kf cargo.Kilnfile, kl cargo.KilnfileLock) (cargo.Kilnfile, error) {
	kf.Stemcell.Version = kl.Stemcell.Version
	for releaseIndex, release := range kf.Releases {
		l, err := kl.FindReleaseWithName(release.Name)
		if err != nil {
			return cargo.Kilnfile{}, fmt.Errorf("release with name %q not found in Kilnfile.lock: %w", release.Name, err)
		}

		kf.Releases[releaseIndex].Version = l.Version
	}

	return kf, nil
}
