package commands

import (
	"errors"

	"github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/jhanda/flags"
)

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
		Description:      "Builds a tile to be uploaded to OpsMan from provided inputs.",
		ShortDescription: "builds a tile",
		Flags:            b.Options,
	}
}

func (b Bake) parseArgs(args []string) (BakeConfig, error) {
	config := BakeConfig{}

	args, err := flags.Parse(&config, args)
	if err != nil {
		panic(err)
	}

	if len(config.ReleaseTarballs) == 0 {
		return config, errors.New("Please specify at least one release tarball with the --release-tarball parameter")
	}

	if config.StemcellTarball == "" {
		return config, errors.New("--stemcell-tarball is a required parameter")
	}

	if config.Handcraft == "" {
		return config, errors.New("--handcraft is a required parameter")
	}

	if config.Version == "" {
		return config, errors.New("--version is a required parameter")
	}

	if config.FinalVersion == "" {
		return config, errors.New("--final-version is a required parameter")
	}

	if config.ProductName == "" {
		return config, errors.New("--product-name is a required parameter")
	}

	if config.FilenamePrefix == "" {
		return config, errors.New("--filename-prefix is a required parameter")
	}

	if config.OutputDir == "" {
		return config, errors.New("--output-dir is a required parameter")
	}

	if len(config.MigrationDirectories) > 0 && len(config.ContentMigrations) > 0 {
		return config, errors.New("cannot build a tile with content migrations and migrations")
	}

	if len(config.ContentMigrations) > 0 && config.BaseContentMigration == "" {
		return config, errors.New("base content migration is required when content migrations are provided")
	}

	if len(config.MigrationDirectories) > 0 && config.BaseContentMigration != "" {
		return config, errors.New("cannot build a tile with a base content migration and migrations")
	}

	return config, nil
}
