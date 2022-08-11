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

	commandSet := jhanda.CommandSet{}

	const (
		bakeCommandName                = "bake"
		cacheReleasesCommandName       = "cache-releases"
		createReleaseNotesCommandName  = "create-release-notes"
		fetchReleasesCommandName       = "fetch-releases"
		findReleaseVersionCommandName  = "find-release-version"
		findStemcellVersionCommandName = "find-stemcell-version"
		publishReleaseCommandName      = "publish-release"
		updateReleaseCommandName       = "update-release"
		updateStemcellCommandName      = "update-stemcell"
		validateCommandName            = "validate"
	)

	// Global Commands
	commandSet["help"] = commands.NewHelp(os.Stdout, globalFlagsUsage, commandSet, map[string][]string{
		"Component Team Commands": {publishReleaseCommandName, updateReleaseCommandName},
		"Tile Commands":           {bakeCommandName, validateCommandName, createReleaseNotesCommandName},
		"Component Commands":      {fetchReleasesCommandName, cacheReleasesCommandName, findReleaseVersionCommandName, findStemcellVersionCommandName, updateStemcellCommandName},
	})
	commandSet["version"] = commands.NewVersion(outLogger, version)

	// Component Team Commands
	commandSet[publishReleaseCommandName] = commands.NewUpdateRelease(outLogger, fs, mrsProvider)
	commandSet[updateReleaseCommandName] = &commands.PublishRelease{
		FS:                    fs,
		Logger:                outLogger,
		ReleaseUploaderFinder: ruFinder,
	}

	// Tile Commands
	commandSet[bakeCommandName] = commands.NewBake(fs, releasesService, outLogger, errLogger)
	commandSet[validateCommandName] = commands.NewValidate(osfs.New(""))
	commandSet[createReleaseNotesCommandName] = commands.NewReleaseNotesCommand()

	// Component Commands
	commandSet[fetchReleasesCommandName] = commands.NewFetchReleases(outLogger, mrsProvider, localReleaseDirectory)
	commandSet[cacheReleasesCommandName] = commands.NewCacheReleases().WithLogger(outLogger)
	commandSet[findReleaseVersionCommandName] = commands.NewFindReleaseVersion(outLogger, mrsProvider)
	commandSet[findStemcellVersionCommandName] = commands.NewFindStemcellVersion(outLogger, pivnetService)
	commandSet[updateStemcellCommandName] = &commands.UpdateStemcell{
		Logger:                     outLogger,
		MultiReleaseSourceProvider: mrsProvider,
		FS:                         osfs.New(""),
	}

	err = commandSet.Execute(command, args)
	if err != nil {
		log.Fatal(err)
	}
}
