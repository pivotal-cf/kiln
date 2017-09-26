package kiln

import "github.com/pivotal-cf/jhanda/flags"

type Application struct {
	argParser argParser
	tileMaker tileMaker
}

type ApplicationConfig struct {
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

//go:generate counterfeiter -o ./fakes/arg_parser.go --fake-name ArgParser . argParser
type argParser interface {
	Parse([]string) (ApplicationConfig, error)
}

//go:generate counterfeiter -o ./fakes/tile_maker.go --fake-name TileMaker . tileMaker
type tileMaker interface {
	Make(ApplicationConfig) error
}

func NewApplication(argParser argParser, tileMaker tileMaker) Application {
	return Application{
		argParser: argParser,
		tileMaker: tileMaker,
	}
}

func (a Application) Run(args []string) error {
	config, err := a.argParser.Parse(args)
	if err != nil {
		return err
	}

	err = a.tileMaker.Make(config)
	if err != nil {
		return err
	}

	return nil
}
