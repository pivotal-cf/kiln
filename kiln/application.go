package kiln

type Application struct {
	argParser argParser
	tileMaker tileMaker
}

type ApplicationConfig struct {
	ReleaseTarballs      StringSlice
	Migrations           StringSlice
	ContentMigrations    StringSlice
	BaseContentMigration string
	StemcellTarball      string
	Handcraft            string
	Version              string
	FinalVersion         string
	Name                 string
	OutputDir            string
	StubReleases         bool
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
