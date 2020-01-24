package main

import (
	"log"
	"os"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/helper"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4/osfs"
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

	filesystem := helper.NewFilesystem()
	fs := osfs.New("")

	zipper := builder.NewZipper()
	interpolator := builder.NewInterpolator()
	tileWriter := builder.NewTileWriter(filesystem, &zipper, errLogger)

	releaseManifestReader := builder.NewReleaseManifestReader(osfs.New(""))
	releasesService := baking.NewReleasesService(errLogger, releaseManifestReader)

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

	localReleaseDirectory := fetcher.NewLocalReleaseDirectory(outLogger, releasesService)

	commandSet := jhanda.CommandSet{}
	commandSet["help"] = commands.NewHelp(os.Stdout, globalFlagsUsage, commandSet)
	commandSet["version"] = commands.NewVersion(outLogger, version)

	releaseSourcesFactory := fetcher.NewReleaseSourceFactory(outLogger)

	commandSet["fetch"] = commands.NewFetch(outLogger, releaseSourcesFactory, localReleaseDirectory)
	commandSet["publish"] = commands.NewPublish(outLogger, errLogger, osfs.New(""))
	commandSet["bake"] = commands.NewBake(
		interpolator,
		tileWriter,
		outLogger,
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

	commandSet["update-stemcell"] = commands.UpdateStemcell{
		StemcellsVersionsService: new(fetcher.Pivnet),
	}

	commandSet["update-release"] = commands.NewUpdateRelease(outLogger, fs, newReleaseDownloaderFactory(), cargo.KilnfileLoader{})

	commandSet["upload-release"] = commands.UploadRelease{
		FS:                     osfs.New(""),
		KilnfileLoader:         cargo.KilnfileLoader{},
		ReleaseUploaderFactory: releaseSourcesFactory,
		Logger:                 log.New(os.Stdout, "", 0),
	}

	err = commandSet.Execute(command, args)
	if err != nil {
		log.Fatal(err)
	}
}

type releaseDownloaderFactory struct{}

func (f releaseDownloaderFactory) ReleaseDownloader(outLogger *log.Logger, kilnfile cargo.Kilnfile, allowOnlyPublishable bool) (commands.ReleaseDownloader, error) {
	releaseSources := fetcher.NewReleaseSourceFactory(outLogger)(kilnfile, allowOnlyPublishable)

	return fetcher.NewReleaseDownloader(releaseSources), nil
}

func newReleaseDownloaderFactory() releaseDownloaderFactory {
	return releaseDownloaderFactory{}
}
