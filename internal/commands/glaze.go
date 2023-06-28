package commands

import (
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Glaze struct {
	Options struct {
		Kilnfile string `short:"kf" long:"kilnfile" default:"Kilnfile"  description:"path to Kilnfile"`
		Undo     bool   `           long:"undo"                         description:"loosens bosh release constraints post-GA based on 'maintenance_version_bump_policy' and 'float_always'"`
	}

	glaze, deGlaze func(kf *cargo.Kilnfile, kl cargo.KilnfileLock) error
}

func NewGlaze() *Glaze {
	return &Glaze{
		glaze:   (*cargo.Kilnfile).Glaze,
		deGlaze: (*cargo.Kilnfile).DeGlaze,
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
	kilnfile, kilnfileLock, err := cargo.ReadKilnfileAndKilnfileLock(kfPath)
	if err != nil {
		return err
	}
	if cmd.Options.Undo {
		err = cmd.deGlaze(&kilnfile, kilnfileLock)
	} else {
		err = cmd.glaze(&kilnfile, kilnfileLock)
	}
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
