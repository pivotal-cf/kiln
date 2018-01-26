package main

import (
	"log"
	"os"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/helper"
	yaml "gopkg.in/yaml.v2"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	var global struct {
		Help bool `short:"h" long:"help" description:"prints this usage information" default:"false"`
	}

	args, err := jhanda.Parse(&global, os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	globalFlagsUsage, err := jhanda.PrintUsage(global)
	if err != nil {
		log.Fatal(err)
	}

	filesystem := helper.NewFilesystem()
	zipper := builder.NewZipper()
	formDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	handcraftReader := builder.NewMetadataReader(filesystem, logger)
	iconEncoder := builder.NewIconEncoder(filesystem)
	instanceGroupDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	jobsDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	propertiesDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	runtimeConfigsDirectoryReader := builder.NewMetadataPartsDirectoryReader()
	releaseManifestReader := builder.NewReleaseManifestReader()
	stemcellManifestReader := builder.NewStemcellManifestReader(filesystem)
	variablesDirectoryReader := builder.NewMetadataPartsDirectoryReaderWithTopLevelKey("variables")
	metadataBuilder := builder.NewMetadataBuilder(
		variablesDirectoryReader,
		handcraftReader,
		logger,
		iconEncoder,
	)
	md5SumCalculator := helper.NewFileMD5SumCalculator()
	interpolator := builder.NewInterpolator()
	tileWriter := builder.NewTileWriter(filesystem, &zipper, logger, md5SumCalculator)
	templateVariablesParser := commands.NewTemplateVariableParser()

	commandSet := jhanda.CommandSet{}
	commandSet["help"] = commands.NewHelp(os.Stdout, globalFlagsUsage, commandSet)
	commandSet["bake"] = commands.NewBake(
		metadataBuilder,
		interpolator,
		tileWriter,
		logger,
		releaseManifestReader,
		stemcellManifestReader,
		formDirectoryReader,
		instanceGroupDirectoryReader,
		jobsDirectoryReader,
		propertiesDirectoryReader,
		runtimeConfigsDirectoryReader,
		yaml.Marshal,
		templateVariablesParser,
	)

	var command string
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}

	if global.Help {
		command = "help"
	}

	if command == "" {
		command = "help"
	}

	err = commandSet.Execute(command, args)
	if err != nil {
		log.Fatal(err)
	}
}
