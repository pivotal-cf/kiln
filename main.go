package main

import (
	"log"
	"os"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/pivnet"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var version = "unknown"

func main() {
	errLogger := log.New(os.Stderr, "", 0)
	outLogger := log.New(os.Stdout, "", 0)

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

	fs := osfs.New("")

	releaseManifestReader := builder.NewReleaseManifestReader(fs)
	releasesService := baking.NewReleasesService(errLogger, releaseManifestReader)
	pivnetService := new(pivnet.Service)
	localReleaseDirectory := component.NewLocalReleaseDirectory(outLogger, releasesService)
	mrsProvider := commands.MultiReleaseSourceProvider(func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) component.MultiReleaseSource {
		repo := component.NewReleaseSourceRepo(kilnfile, outLogger)
		return repo.Filter(allowOnlyPublishable)
	})
	ruFinder := commands.ReleaseUploaderFinder(func(kilnfile cargo.Kilnfile, sourceID string) (component.ReleaseUploader, error) {
		repo := component.NewReleaseSourceRepo(kilnfile, outLogger)
		return repo.FindReleaseUploader(sourceID)
	})
	rpFinder := commands.RemotePatherFinder(func(kilnfile cargo.Kilnfile, sourceID string) (component.RemotePather, error) {
		repo := component.NewReleaseSourceRepo(kilnfile, outLogger)
		return repo.FindRemotePather(sourceID)
	})

	commandSet := jhanda.CommandSet{}
<<<<<<< HEAD

	if command == "easy-bake" {
		commandSet["fetch"] = commands.NewFetch(outLogger, mrsProvider, localReleaseDirectory)
		
		fetchArgs := []string{"--allow-only-publishable"}
		err = commandSet.Execute("fetch", fetchArgs)
		if err != nil {
			log.Fatal(err)
		}

		commandSet["easy-bake"] = commands.NewEasyBake(outLogger, fs, releasesService)
		err = commandSet.Execute(command, args)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	commandSet["test"] = commands.NewTestTile(outLogger)
	commandSet["help"] = commands.NewHelp(os.Stdout, globalFlagsUsage, commandSet)
	commandSet["version"] = commands.NewVersion(outLogger, version)
	commandSet["bake"] = commands.NewBake(fs, releasesService, outLogger, errLogger)
	commandSet["update-release"] = commands.NewUpdateRelease(outLogger, fs, mrsProvider)
	commandSet["fetch"] = commands.NewFetch(outLogger, mrsProvider, localReleaseDirectory)
	commandSet["upload-release"] = commands.UploadRelease{
		FS:                    fs,
		Logger:                outLogger,
		ReleaseUploaderFinder: ruFinder,
	}
	commandSet["sync-with-local"] = commands.NewSyncWithLocal(fs, localReleaseDirectory, rpFinder, outLogger)
	commandSet["publish"] = commands.NewPublish(outLogger, errLogger, osfs.New(""))

	commandSet["update-stemcell"] = commands.UpdateStemcell{
		Logger:                     outLogger,
		MultiReleaseSourceProvider: mrsProvider,
		FS:                         osfs.New(""),
	}

	commandSet["glaze"] = new(commands.Glaze)

	commandSet["find-release-version"] = commands.NewFindReleaseVersion(outLogger, mrsProvider)

	commandSet["find-stemcell-version"] = commands.NewFindStemcellVersion(outLogger, pivnetService)

	commandSet["cache-compiled-releases"] = commands.NewCacheCompiledReleases().WithLogger(outLogger)

	commandSet["validate"] = commands.NewValidate(osfs.New(""))
	commandSet["release-notes"], err = commands.NewReleaseNotesCommand()
	if err != nil {
		log.Fatal(err)
	}

	err = commandSet.Execute(command, args)
	if err != nil {
		log.Fatal(err)
	}
}
