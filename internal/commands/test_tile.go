package commands

import (
	"context"
	_ "embed"
	"io"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/test"
)

type TileTestFunction func(ctx context.Context, w io.Writer, configuration test.Configuration) error

type TileTest struct {
	Options struct {
		TilePath   string `             long:"tile-path"                default:"."                             description:"Path to the Tile directory (e.g., ~/workspace/tas/ist)."`
		Verbose    bool   `short:"v"    long:"verbose"                  default:"false"                         description:"Print info lines. This doesn't affect Ginkgo output."`
		Manifest   bool   `             long:"manifest"                 default:"false"                         description:"Focus the Manifest tests."`
		Migrations bool   `             long:"migrations"               default:"false"                         description:"Focus the Migration tests."`
		Stability  bool   `             long:"stability"                default:"false"                         description:"Focus the Stability tests."`

		EnvironmentVars []string `short:"e"    long:"environment-variable"                                             description:"Pass environment variable to the test suites. For example --stability -e 'PRODUCT=srt'."`
		GingkoFlags     string   `             long:"ginkgo-flags"             default:"-r -p -slowSpecThreshold 15"   description:"Flags to pass to the Ginkgo Manifest and Stability test suites."`
	}
	function TileTestFunction
	output   io.Writer
}

func NewTileTest() TileTest {
	return TileTest{function: test.Run, output: os.Stdout}
}

func NewTileTestWithCollaborators(w io.Writer, fn TileTestFunction) TileTest {
	return TileTest{function: fn, output: w}
}

func (cmd TileTest) Execute(args []string) error {
	ctx := context.Background()

	if _, err := jhanda.Parse(&cmd.Options, args); err != nil {
		return err
	}

	configuration, err := cmd.configuration()
	if err != nil {
		return err
	}

	w := cmd.output
	if !cmd.Options.Verbose {
		w = io.Discard
	}

	return cmd.function(ctx, w, configuration)
}

func (cmd TileTest) configuration() (test.Configuration, error) {
	absPath, absErr := filepath.Abs(cmd.Options.TilePath)
	return test.Configuration{
		AbsoluteTileDirectory: absPath,

		RunAll:        !cmd.Options.Migrations && !cmd.Options.Manifest && !cmd.Options.Stability,
		RunManifest:   cmd.Options.Manifest,
		RunMetadata:   cmd.Options.Stability,
		RunMigrations: cmd.Options.Migrations,

		GinkgoFlags: cmd.Options.GingkoFlags,
		Environment: cmd.Options.EnvironmentVars,
	}, absErr
}

func (cmd TileTest) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Run the Manifest, Migrations, and Stability tests for a Tile in a Docker container. Requires a Docker daemon to be running and ssh keys with access to Ops Manager's Git repository. For non-interactive use, either set the environment variable SSH_PASSWORD, or `ssh add` your identity before running.",
		ShortDescription: "Runs unit tests for a Tile.",
		Flags:            cmd.Options,
	}
}
