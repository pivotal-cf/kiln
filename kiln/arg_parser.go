package kiln

import (
	"errors"
	"flag"
)

type ArgParser struct{}

func NewArgParser() ArgParser {
	return ArgParser{}
}

func (a ArgParser) Parse(args []string) (ApplicationConfig, error) {
	cfg := ApplicationConfig{}

	flagSet := flag.NewFlagSet("kiln", flag.ExitOnError)
	flagSet.Var(&cfg.ReleaseTarballs, "release-tarball", "Path to release tarball")
	flagSet.Var(&cfg.Migrations, "migration", "Path to a migration file")
	flagSet.Var(&cfg.ContentMigrations, "content-migration", "Path to a content_migration file")
	flagSet.StringVar(&cfg.BaseContentMigration, "base-content-migration", "", "Path to the base content_migration file")
	flagSet.StringVar(&cfg.StemcellTarball, "stemcell-tarball", "", "Path to stemcell tarball")
	flagSet.StringVar(&cfg.Handcraft, "handcraft", "", "Path to handcraft.yml")
	flagSet.StringVar(&cfg.Version, "version", "", "version number to be used for file name")
	flagSet.StringVar(&cfg.FinalVersion, "final-version", "", "version number to be used in tile metadata")
	flagSet.StringVar(&cfg.ProductName, "product-name", "", "name of the product")
	flagSet.StringVar(&cfg.FilenamePrefix, "filename-prefix", "", "product filename prefix")
	flagSet.StringVar(&cfg.OutputDir, "output-dir", "", "Directory where metadata file should be output")
	flagSet.BoolVar(&cfg.StubReleases, "stub-releases", false, "stubs release tarballs with empty files")
	flagSet.Parse(args)

	if len(cfg.ReleaseTarballs) == 0 {
		return cfg, errors.New("Please specify at least one release tarball with the --release-tarball parameter")
	}

	if cfg.StemcellTarball == "" {
		return cfg, errors.New("--stemcell-tarball is a required parameter")
	}

	if cfg.Handcraft == "" {
		return cfg, errors.New("--handcraft is a required parameter")
	}

	if cfg.Version == "" {
		return cfg, errors.New("--version is a required parameter")
	}

	if cfg.FinalVersion == "" {
		return cfg, errors.New("--final-version is a required parameter")
	}

	if cfg.ProductName == "" {
		return cfg, errors.New("--product-name is a required parameter")
	}

	if cfg.FilenamePrefix == "" {
		return cfg, errors.New("--filename-prefix is a required parameter")
	}

	if cfg.OutputDir == "" {
		return cfg, errors.New("--output-dir is a required parameter")
	}

	if len(cfg.Migrations) > 0 && len(cfg.ContentMigrations) > 0 {
		return cfg, errors.New("cannot build a tile with content migrations and migrations")
	}

	if len(cfg.ContentMigrations) > 0 && cfg.BaseContentMigration == "" {
		return cfg, errors.New("base content migration is required when content migrations are provided")
	}

	if len(cfg.Migrations) > 0 && cfg.BaseContentMigration != "" {
		return cfg, errors.New("cannot build a tile with a base content migration and migrations")
	}

	return cfg, nil
}
