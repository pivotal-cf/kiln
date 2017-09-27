package commands

import (
	"github.com/pivotal-cf/jhanda/flags"
	"github.com/pivotal-cf/kiln/kiln"
)

type Bake struct {
	argParser argParser
	tileMaker tileMaker
	Options   struct {
		ReleaseTarballs      flags.StringSlice `short:"rt"   long:"release-tarball"         description:""`
		Migrations           flags.StringSlice `short:"m"    long:"migration"               description:""`
		ContentMigrations    flags.StringSlice `short:"cm"   long:"content-migration"       description:""`
		BaseContentMigration string            `short:"bcm"  long:"base-content-migration"  description:""`
		StemcellTarball      string            `short:"st"   long:"stemcell-tarball"        description:""`
		Handcraft            string            `short:"h"    long:"handcraft"               description:""`
		Version              string            `short:"v"    long:"version"                 description:""`
		FinalVersion         string            `short:"fv"   long:"final-version"           description:""`
		ProductName          string            `short:"pn"   long:"product-name"            description:""`
		FilenamePrefix       string            `short:"fp"   long:"filename-prefix"         description:""`
		OutputDir            string            `short:"o"    long:"output-dir"              description:""`
		StubReleases         bool              `short:"sr"   long:"stub-releases"           description:""`
	}
}

type argParser interface {
	Parse([]string) (kiln.ApplicationConfig, error)
}

type tileMaker interface {
	Make(kiln.ApplicationConfig) error
}

func NewBake(argParser argParser, tileMaker tileMaker) Bake {
	return Bake{
		argParser: argParser,
		tileMaker: tileMaker,
	}
}

func (b Bake) Execute(args []string) error {
	config, err := b.argParser.Parse(args)
	if err != nil {
		panic(err)
	}

	err = b.tileMaker.Make(config)
	if err != nil {
		return err
	}

	return nil

}

func (b Bake) Usage() Usage {
	return Usage{
		Description:      "Builds a tile to be uploaded to OpsMan from provided inputs.",
		ShortDescription: "builds a tile",
		Flags:            b.Options,
	}
}
