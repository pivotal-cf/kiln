package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/test"
)

//counterfeiter:generate -o ./fakes/test_tile_function.go --fake-name TestTileFunction . TileTestFunction
type TileTestFunction func(ctx context.Context, w io.Writer, configuration test.Configuration) error

type TileTest struct {
	Options struct {
		TilePath   string `             long:"tile-path"                default:"."                             description:"Path to the Tile directory (e.g. ~/workspace/tas/ist)."`
		Verbose    bool   `short:"v"    long:"verbose"                  default:"true"                          description:"Print extra details such as the container ID. This doesn't affect Ginkgo output."`
		Silent     bool   `short:"s"    long:"silent"                   default:"false"                         description:"Hide info lines. This doesn't affect Ginkgo output."`
		Manifest   bool   `             long:"manifest"                 default:"false"                         description:"Focus the Manifest tests."`
		Migrations bool   `             long:"migrations"               default:"false"                         description:"Focus the Migration tests."`
		Stability  bool   `             long:"stability"                default:"false"                         description:"Focus the Stability tests."`

		EnvironmentVars []string `short:"e"    long:"environment-variable"                                             description:"Pass environment variables to the test suites (e.g. -e 'PRODUCT=srt'). Include -e ARTIFACTORY_USERNAME=... and -e ARTIFACTORY_PASSWORD=... unless they are exported."`
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
	if cmd.Options.Silent {
		w = io.Discard
	}

	return cmd.function(ctx, w, configuration)
}

func (cmd TileTest) configuration() (test.Configuration, error) {
	absPath, absErr := filepath.Abs(cmd.Options.TilePath)
	if _, err := os.Stat(absPath); err != nil {
		return test.Configuration{}, fmt.Errorf("failed to get information about --tile-path: %w", err)
	}
	if _, _, err := test.RequiredArtifactoryCredentials(cmd.Options.EnvironmentVars); err != nil {
		return test.Configuration{}, err
	}
	cfg := test.Configuration{
		AbsoluteTileDirectory: absPath,

		RunAll:        !cmd.Options.Migrations && !cmd.Options.Manifest && !cmd.Options.Stability,
		RunManifest:   cmd.Options.Manifest,
		RunMetadata:   cmd.Options.Stability,
		RunMigrations: cmd.Options.Migrations,

		GinkgoFlags: cmd.Options.GingkoFlags,
		Environment: cmd.Options.EnvironmentVars,
		Verbose:     cmd.Options.Verbose,
	}
	return cfg, absErr
}

func (cmd TileTest) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Run the Manifest, Migrations, and Stability tests for a Tile in a Docker container. Requires a Docker daemon. Requires ARTIFACTORY_USERNAME and ARTIFACTORY_PASSWORD (via -e or your environment) for the test image build and ops-manifest gem. Kiln passes the same credentials to the Docker daemon for pulling base images from docker-virtual (no separate docker login needed for kiln test).",
		ShortDescription: "Runs unit tests for a Tile.",
		Flags:            cmd.Options,
	}
}
