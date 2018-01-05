package main

import (
	"log"
	"os"

	jhandacommands "github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/jhanda/flags"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/helper"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	var global struct {
		Help bool `short:"h" long:"help"                description:"prints this usage information"                        default:"false"`
	}

	args, err := flags.Parse(&global, os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	globalFlagsUsage, err := flags.Usage(global)
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
	propertiesDirectoryReader := builder.NewMetadataPartsDirectoryReaderWithTopLevelKey("property_blueprints")
	releaseManifestReader := builder.NewReleaseManifestReader()
	runtimeConfigsDirectoryReader := builder.NewMetadataPartsDirectoryReaderWithTopLevelKey("runtime_configs")
	stemcellManifestReader := builder.NewStemcellManifestReader(filesystem)
	variablesDirectoryReader := builder.NewMetadataPartsDirectoryReaderWithTopLevelKey("variables")
	metadataBuilder := builder.NewMetadataBuilder(
		instanceGroupDirectoryReader,
		jobsDirectoryReader,
		propertiesDirectoryReader,
		runtimeConfigsDirectoryReader,
		variablesDirectoryReader,
		handcraftReader,
		logger,
		iconEncoder,
	)
	md5SumCalculator := helper.NewFileMD5SumCalculator()
	interpolator := builder.NewInterpolator()
	tileWriter := builder.NewTileWriter(filesystem, &zipper, logger, md5SumCalculator)

	commandSet := jhandacommands.Set{}
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
