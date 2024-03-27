package commands

import (
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/pkg/bake"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type New struct {
	Options struct {
		Dir              string `long:"directory"         description:"path to directory where source should be written" default:"."`
		Tile             string `long:"tile"              description:"tile path"               required:"true"`
		Slug             string `long:"product-slug"      description:"a TanzuNet product slug"`
		KilnfileTemplate string `long:"kilnfile-template" description:"a Kilnfile template sets up some reasonable Kilnfile defaults one of [artifactory, bosh.io]" default:"bosh.io"`
	}
}

func (n *New) Execute(args []string) error {
	k, err := n.Setup(args)
	if err != nil {
		return err
	}
	k.Slug = n.Options.Slug
	return bake.New(n.Options.Dir, n.Options.Tile, k)
}

func (n *New) Setup(args []string) (cargo.Kilnfile, error) {
	if _, err := jhanda.Parse(&n.Options, args); err != nil {
		return cargo.Kilnfile{}, err
	}
	k, err := cargo.KilnfileTemplate(n.Options.KilnfileTemplate)
	if err != nil {
		return cargo.Kilnfile{}, err
	}
	return k, nil
}

func (n *New) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "generate tile source from a tile",
		ShortDescription: "generate source",
		Flags:            n.Options,
	}
}
