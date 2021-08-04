package main

import (
	"log"
	"os"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/fetcher"
	"github.com/pivotal-cf/kiln/internal/helper"
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
	pivnetService := fetcher.CreateNewPivnetService()
	localReleaseDirectory := fetcher.NewLocalReleaseDirectory(outLogger, releasesService)
	mrsProvider := commands.MultiReleaseSourceProvider(func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) fetcher.MultiReleaseSource {
		repo := fetcher.NewReleaseSourceRepo(kilnfile, outLogger)
		return repo.MultiReleaseSource(allowOnlyPublishable)
	})
	ruFinder := commands.ReleaseUploaderFinder(func(kilnfile cargo.Kilnfile, sourceID string) (fetcher.ReleaseUploader, error) {
		repo := fetcher.NewReleaseSourceRepo(kilnfile, outLogger)
		return repo.FindReleaseUploader(sourceID)
	})
	rpFinder := commands.RemotePatherFinder(func(kilnfile cargo.Kilnfile, sourceID string) (fetcher.RemotePather, error) {
		repo := fetcher.NewReleaseSourceRepo(kilnfile, outLogger)
		return repo.FindRemotePather(sourceID)
	})

	commandSet := jhanda.CommandSet{}
	commandSet["help"] = commands.NewHelp(os.Stdout, globalFlagsUsage, commandSet)
	commandSet["version"] = commands.NewVersion(outLogger, version)
	commandSet["bake"] = bakeCommand(fs, releasesService, outLogger, errLogger)
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

	commandSet["find-release-version"] = commands.NewFindReleaseVersion(outLogger, mrsProvider)

	commandSet["find-stemcell-version"] = commands.NewFindStemcellVersion(outLogger, pivnetService)

	commandSet["compile-built-releases"] = commands.CompileBuiltReleases{
		BoshDirectorFactory:        commands.BoshDirectorFactory,
		Logger:                     outLogger,
		MultiReleaseSourceProvider: mrsProvider,
		ReleaseUploaderFinder:      ruFinder,
	}

	commandSet["validate"] = commands.Validate{}
	commandSet["pre-process"] = commands.PreProcess{}

	err = commandSet.Execute(command, args)
	if err != nil {
		log.Fatal(err)
	}
}

func bakeCommand(fs billy.Filesystem, releasesService baking.ReleasesService, outLogger *log.Logger, errLogger *log.Logger) commands.Bake {
	filesystem := helper.NewFilesystem()
	zipper := builder.NewZipper()
	interpolator := builder.NewInterpolator()
	tileWriter := builder.NewTileWriter(filesystem, &zipper, errLogger)

	stemcellManifestReader := builder.NewStemcellManifestReader(filesystem)
	stemcellService := baking.NewStemcellService(errLogger, stemcellManifestReader)

	templateVariablesService := baking.NewTemplateVariablesService(fs)

	boshVariableDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	boshVariablesService := baking.NewBOSHVariablesService(errLogger, boshVariableDirectoryReader)

	formDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	formsService := baking.NewFormsService(errLogger, formDirectoryReader)

	instanceGroupDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	instanceGroupsService := baking.NewInstanceGroupsService(errLogger, instanceGroupDirectoryReader)

	jobsDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	jobsService := baking.NewJobsService(errLogger, jobsDirectoryReader)

	propertiesDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	propertiesService := baking.NewPropertiesService(errLogger, propertiesDirectoryReader)

	runtimeConfigsDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	runtimeConfigsService := baking.NewRuntimeConfigsService(errLogger, runtimeConfigsDirectoryReader)

	iconService := baking.NewIconService(errLogger)

	metadataService := baking.NewMetadataService()
	checksummer := baking.NewChecksummer(errLogger)

	return commands.NewBake(
		interpolator,
		tileWriter,
		outLogger,
		errLogger,
		templateVariablesService,
		boshVariablesService,
		releasesService,
		stemcellService,
		formsService,
		instanceGroupsService,
		jobsService,
		propertiesService,
		runtimeConfigsService,
		iconService,
		metadataService,
		checksummer,
	)
}
