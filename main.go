package main

import (
	"log"
	"os"

	jhandacommands "github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/jhanda/flags"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/helper"
	"github.com/pivotal-cf/kiln/kiln"
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
	releaseManifestReader := builder.NewReleaseManifestReader(filesystem)
	runtimeConfigsDirectoryReader := builder.NewMetadataPartsDirectoryReader(filesystem, "runtime_configs")
	variablesDirectoryReader := builder.NewMetadataPartsDirectoryReader(filesystem, "variables")
	stemcellManifestReader := builder.NewStemcellManifestReader(filesystem)
	handcraftReader := builder.NewMetadataReader(filesystem, logger)
	metadataBuilder := builder.NewMetadataBuilder(
		releaseManifestReader,
		runtimeConfigsDirectoryReader,
		variablesDirectoryReader,
		stemcellManifestReader,
		handcraftReader,
		logger,
	)
	md5SumCalculator := helper.NewFileMD5SumCalculator()
	tileWriter := builder.NewTileWriter(filesystem, &zipper, logger, md5SumCalculator)
	tileMaker := kiln.NewTileMaker(metadataBuilder, tileWriter, logger)

	commandSet := jhandacommands.Set{}
	commandSet["help"] = commands.NewHelp(os.Stdout, globalFlagsUsage, commandSet)
	commandSet["bake"] = commands.NewBake(tileMaker)

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
