package main

import (
	"fmt"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/helper"
	"github.com/pivotal-cf/kiln/internal/baking"
)

var version = "unknown"

type globalOptions struct {
	Help           bool     `short:"h"  long:"help"           description:"prints this usage information"   default:"false"`
	Version        bool     `short:"v"  long:"version"        description:"prints the kiln release version" default:"false"`
	Kilnfile       string   `short:"kf" long:"kilnfile"       description:"path to Kilnfile"                default:"Kilnfile"`
	VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`
	Variables      []string `short:"vr" long:"variable"       description:"variable in key=value format"`
}

func main() {
	errLogger := log.New(os.Stderr, "", 0)
	outLogger := log.New(os.Stdout, "", 0)

	var global globalOptions

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
	zipper := builder.NewZipper()
	interpolator := builder.NewInterpolator()
	tileWriter := builder.NewTileWriter(filesystem, &zipper, errLogger)

	releaseManifestReader := builder.NewReleaseManifestReader()
	releasesService := baking.NewReleasesService(errLogger, releaseManifestReader)

	stemcellManifestReader := builder.NewStemcellManifestReader(filesystem)
	stemcellService := baking.NewStemcellService(errLogger, stemcellManifestReader)

	templateVariablesService := baking.NewTemplateVariablesService()
	kilnfile, kilnfileLock, err := loadKilnfiles(templateVariablesService, global, outLogger)

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

	releaseSourcesFactory := fetcher.NewReleaseSourcesFactory(outLogger)

	commandSet["fetch"] = commands.NewFetch(outLogger, kilnfile, kilnfileLock, releaseSourcesFactory, localReleaseDirectory)
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

	commandSet["update"] = commands.Update{
		StemcellsVersionsService: new(fetcher.Pivnet),
	}

	err = commandSet.Execute(command, args)
	if err != nil {
		log.Fatal(err)
	}
}

type ConfigFileError struct {
	HumanReadableConfigFileName string
	err                         error
}

func (err ConfigFileError) Unwrap() error {
	return err.err
}

func (err ConfigFileError) Error() string {
	return fmt.Sprintf("encountered a configuration file error with %s: %s", err.HumanReadableConfigFileName, err.err.Error())
}

func loadKilnfiles(templateVariablesService baking.TemplateVariablesService, global globalOptions, logger *log.Logger) (cargo.Kilnfile, cargo.KilnfileLock, error) {
	templateVariables, err := templateVariablesService.FromPathsAndPairs(global.VariablesFiles, global.Variables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, fmt.Errorf("failed to parse template variables: %s", err)
	}

	kilnfileYAML, err := ioutil.ReadFile(global.Kilnfile)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}
	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, kilnfileYAML)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, ConfigFileError{err: err, HumanReadableConfigFileName: "interpolating variable files with Kilnfile"}
	}

	logger.Println("getting release information from " + global.Kilnfile)
	var kilnfile cargo.Kilnfile
	err = yaml.Unmarshal(interpolatedMetadata, &kilnfile)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, ConfigFileError{err: err, HumanReadableConfigFileName: "Kilnfile specification " + global.Kilnfile}
	}

	logger.Println("getting release information from Kilnfile.lock")
	lockFileName := fmt.Sprintf("%s.lock", global.Kilnfile)
	lockFile, err := os.Open(lockFileName)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}
	defer lockFile.Close()

	var kilnfileLock cargo.KilnfileLock
	err = yaml.NewDecoder(lockFile).Decode(&kilnfileLock)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, ConfigFileError{err: err, HumanReadableConfigFileName: "Kilnfile.lock " + lockFileName}
	}

	return kilnfile, kilnfileLock, nil
}
