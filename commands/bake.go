package commands

import (
	"errors"

	"github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/jhanda/flags"
)

type BakeConfig struct {
	ReleaseDirectories       flags.StringSlice `short:"rd"   long:"releases-directory"         description:"path to a directory containing release tarballs"`
	MigrationDirectories     flags.StringSlice `short:"md"   long:"migrations-directory"       description:"path to a directory containing migrations"`
	RuntimeConfigDirectories flags.StringSlice `short:"rcd"  long:"runtime-configs-directory"  description:"path to a directory containing runtime configs"`
	VariableDirectories      flags.StringSlice `short:"vd"   long:"variables-directory"        description:"path to a directory containing variables"`
	EmbedPaths               flags.StringSlice `short:"e"    long:"embed"                      description:"path to files to include in the tile /embed directory"`
	StemcellTarball          string            `short:"st"   long:"stemcell-tarball"           description:"path to a stemcell tarball"`
	Metadata                 string            `short:"m"    long:"metadata"                   description:"path to the metadata file"`
	Version                  string            `short:"v"    long:"version"                    description:"version of the tile"`
	OutputFile               string            `short:"o"    long:"output-file"                description:"path to where the tile will be output"`
	StubReleases             bool              `short:"sr"   long:"stub-releases"              description:"skips importing release tarballs into the tile"`
}

type Bake struct {
	tileMaker tileMaker
	Options   BakeConfig
}

//go:generate counterfeiter -o ./fakes/tile_maker.go --fake-name TileMaker . tileMaker
type tileMaker interface {
	Make(BakeConfig) error
}

func NewBake(tileMaker tileMaker) Bake {
	return Bake{
		tileMaker: tileMaker,
	}
}

func (b Bake) Execute(args []string) error {
	config, err := b.parseArgs(args)
	if err != nil {
		return err
	}

	err = b.tileMaker.Make(config)
	if err != nil {
		return err
	}

	return nil
}

func (b Bake) Usage() commands.Usage {
	return commands.Usage{
		Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
		ShortDescription: "bakes a tile",
		Flags:            b.Options,
	}
}

func (b Bake) parseArgs(args []string) (BakeConfig, error) {
	config := BakeConfig{}

	args, err := flags.Parse(&config, args)
	if err != nil {
		panic(err)
	}

	if len(config.ReleaseDirectories) == 0 {
		return config, errors.New("Please specify release tarballs directory with the --releases-directory parameter")
	}

	if config.StemcellTarball == "" {
		return config, errors.New("--stemcell-tarball is a required parameter")
	}

	if config.Metadata == "" {
		return config, errors.New("--metadata is a required parameter")
	}

	if config.Version == "" {
		return config, errors.New("--version is a required parameter")
	}

	if config.OutputFile == "" {
		return config, errors.New("--output-file is a required parameter")
	}

	return config, nil
}
