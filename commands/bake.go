package commands

import (
	"errors"

	"github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/jhanda/flags"
)

type BakeConfig struct {
	FormDirectories          flags.StringSlice `short:"f"    long:"forms-directory"            description:"path to a directory containing forms"`
	InstanceGroupDirectories flags.StringSlice `short:"ig"   long:"instance-groups-directory"  description:"path to a directory containing instance groups"`
	JobDirectories           flags.StringSlice `short:"j"    long:"jobs-directory"             description:"path to a directory containing jobs"`
	EmbedPaths               flags.StringSlice `short:"e"    long:"embed"                      description:"path to files to include in the tile /embed directory"`
	IconPath                 string            `short:"i"    long:"icon"                       description:"path to icon file"`
	Metadata                 string            `short:"m"    long:"metadata"                   description:"path to the metadata file"`
	MigrationDirectories     flags.StringSlice `short:"md"   long:"migrations-directory"       description:"path to a directory containing migrations"`
	OutputFile               string            `short:"o"    long:"output-file"                description:"path to where the tile will be output"`
	ReleaseDirectories       flags.StringSlice `short:"rd"   long:"releases-directory"         description:"path to a directory containing release tarballs"`
	RuntimeConfigDirectories flags.StringSlice `short:"rcd"  long:"runtime-configs-directory"  description:"path to a directory containing runtime configs"`
	StemcellTarball          string            `short:"st"   long:"stemcell-tarball"           description:"path to a stemcell tarball"`
	StubReleases             bool              `short:"sr"   long:"stub-releases"              description:"skips importing release tarballs into the tile"`
	VariableDirectories      flags.StringSlice `short:"vd"   long:"variables-directory"        description:"path to a directory containing variables"`
	Version                  string            `short:"v"    long:"version"                    description:"version of the tile"`
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

	if config.IconPath == "" {
		return config, errors.New("--icon is a required parameter")
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

	if len(config.InstanceGroupDirectories) == 0 && len(config.JobDirectories) > 0 {
		return config, errors.New("--jobs-directory flag requires --instance-groups-directory to also be specified")
	}

	return config, nil
}
