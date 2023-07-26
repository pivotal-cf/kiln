package commands

import (
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type (
	glazeCommandOptions struct {
		Kilnfile string `short:"kf" long:"kilnfile" default:"Kilnfile"  description:"path to Kilnfile"`
	}
	Glaze struct {
		Options glazeCommandOptions
	}
	DeGlaze struct {
		Options glazeCommandOptions
	}
)

func (cmd *Glaze) Execute(args []string) error {
	return glazeCommandExecute(cmd.Options, args, (*cargo.Kilnfile).Glaze)
}

func (cmd *Glaze) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command locks all the components.",
		ShortDescription: "Pin versions in Kilnfile to match lock.",
	}
}

func (cmd *DeGlaze) Execute(args []string) error {
	return glazeCommandExecute(cmd.Options, args, (*cargo.Kilnfile).DeGlaze)
}

func (cmd *DeGlaze) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command unlocks all the components.",
		ShortDescription: "Unpin version constraints in Kilnfile based on de_glaze_behavior.",
	}
}

func glazeCommandExecute(options glazeCommandOptions, args []string, fn func(kilnfile *cargo.Kilnfile, lock cargo.KilnfileLock) error) error {
	_, err := jhanda.Parse(&options, args)
	if err != nil {
		return err
	}
	kfPath, err := cargo.ResolveKilnfilePath(options.Kilnfile)
	if err != nil {
		return err
	}
	kilnfile, kilnfileLock, err := cargo.ReadKilnfileAndKilnfileLock(kfPath)
	if err != nil {
		return err
	}
	if err := fn(&kilnfile, kilnfileLock); err != nil {
		return err
	}
	return cargo.WriteKilnfile(kfPath, kilnfile)
}
