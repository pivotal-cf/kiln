package main

import (
	"github.com/jessevdk/go-flags"
	"log"
	"os"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4/osfs"
)

var version = "unknown"

var RootCmd struct {
	Options struct {
		Kilnfile       string   `long:"kilnfile"       short:"k"  default:"Kilnfile" description:"path to Kilnfile"`
		VariablesFiles []string `long:"variables-file" short:"V"                     description:"path to variables file"`
		Variables      []string `long:"variable"       short:"v"                     description:"variable in key=value format"`
	}

	Commands struct {
		Bake           commands.BakeCmd           `command:"bake"            description:"bakes a tile"                                     long-description:"Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager"`
		Fetch          commands.FetchCmd          `command:"fetch"           description:"fetches releases"                                 long-description:"Fetches releases listed in Kilnfile.lock from S3 and downloads it locally"`
		Publish        commands.PublishCmd        `command:"publish"         description:"publish tile on Pivnet"                           long-description:"UpdateStemcell release date, end of general support date, and license files for a tile on Pivnet"`
		SyncWithLocal  commands.SyncWithLocalCmd  `command:"sync-with-local" description:"update the Kilnfile.lock based on local releases" long-description:"Update the Kilnfile.lock based on the local releases directory. Assume the given release source"`
		UpdateRelease  commands.UpdateReleaseCmd  `command:"update-release"  description:"bumps a release to a new version"                 long-description:"Bumps a release to a new version in Kilnfile.lock"`
		UpdateStemcell commands.UpdateStemcellCmd `command:"update-stemcell" description:"updates Kilnfile.lock with stemcell info"         long-description:"Updates stemcell_criteria and release information in Kilnfile.lock"`
		UploadRelease  commands.UploadReleaseCmd  `command:"upload-release"  description:"uploads a BOSH release to an s3 release_source"   long-description:"Uploads a BOSH Release to an S3 release source for use in kiln fetch"`
	}
}

func main() {
	errLogger := log.New(os.Stderr, "", 0)
	outLogger := log.New(os.Stdout, "", 0)

	fs := osfs.New("")

	releaseManifestReader := builder.NewReleaseManifestReader(fs)
	releasesService := baking.NewReleasesService(errLogger, releaseManifestReader)
	localReleaseDirectory := fetcher.NewLocalReleaseDirectory(outLogger, releasesService)

	type Cmd interface {
		Runner(commands.Dependencies) (commands.CommandRunner, error)
	}

	var _ Cmd = RootCmd.Commands.Bake
	var _ Cmd = RootCmd.Commands.Fetch
	var _ Cmd = RootCmd.Commands.Publish
	var _ Cmd = RootCmd.Commands.SyncWithLocal
	var _ Cmd = RootCmd.Commands.UpdateRelease
	var _ Cmd = RootCmd.Commands.UpdateStemcell
	var _ Cmd = RootCmd.Commands.UploadRelease

	root := flags.NewParser(&RootCmd, flags.Default)

	root.CommandHandler = func(command flags.Commander, args []string) error {
		kilnfilePath := RootCmd.Options.Kilnfile
		kilnfileLockPath := kilnfilePath + ".lock"
		variables := RootCmd.Options.Variables
		variablesFiles := RootCmd.Options.VariablesFiles

		loader := cargo.KilnfileLoader{}
		kf, kfl, err := loader.LoadKilnfiles(fs, kilnfilePath, variablesFiles, variables)
		if err != nil {
			return err
		}
		repo := fetcher.NewReleaseSourceRepo(kf, outLogger)

		cmd := command.(Cmd)
		runner, err := cmd.Runner(commands.Dependencies{
			Kilnfile:              kf,
			KilnfileLock:          kfl,
			KilnfilePath:          kilnfilePath,
			KilnfileLockPath:      kilnfileLockPath,
			Variables:             variables,
			VariablesFiles:        variablesFiles,
			ReleaseSourceRepo:     repo,
			OutLogger:             outLogger,
			ErrLogger:             errLogger,
			LocalReleaseDirectory: localReleaseDirectory,
			Filesystem:            fs,
		})
		if err != nil {
			return err
		}

		return runner.Run(args)
	}

	_, err := root.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return
		}
		log.Fatal(err)
	}

	return

	var global struct {
		Help    bool `short:"h" long:"help"    description:"prints this usage information"   default:"false"`
		Version bool `short:"v" long:"version" description:"prints the kiln release version" default:"false"`
	}

	args, err := jhanda.Parse(&global, os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	globalFlagsUsage, err := jhanda.PrintUsage(global)
	if err != nil {
		log.Fatal(err)
	}

	var command string
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}

	if global.Version {
		command = "version"
	}

	if global.Help {
		command = "help"
	}

	if command == "" {
		command = "help"
	}

	commandSet := jhanda.CommandSet{}
	commandSet["help"] = commands.NewHelp(os.Stdout, globalFlagsUsage, commandSet)
	commandSet["version"] = commands.NewVersion(outLogger, version)

	err = commandSet.Execute(command, args)
	if err != nil {
		log.Fatal(err)
	}
}
