package main

import (
	"log"
	"os"

	jhandacommands "github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/helper"
	"github.com/pivotal-cf/kiln/kiln"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	filesystem := helper.NewFilesystem()
	zipper := builder.NewZipper()
	releaseManifestReader := builder.NewReleaseManifestReader(filesystem)
	stemcellManifestReader := builder.NewStemcellManifestReader(filesystem)
	handcraftReader := builder.NewHandcraftReader(filesystem, logger)
	metadataBuilder := builder.NewMetadataBuilder(releaseManifestReader, stemcellManifestReader, handcraftReader, logger)
	contentMigrationBuilder := builder.NewContentMigrationBuilder(logger)
	md5SumCalculator := helper.NewFileMD5SumCalculator()
	tileWriter := builder.NewTileWriter(filesystem, &zipper, contentMigrationBuilder, logger, md5SumCalculator)
	tileMaker := kiln.NewTileMaker(metadataBuilder, tileWriter, logger)

	commandSet := jhandacommands.Set{}
	commandSet["bake"] = commands.NewBake(tileMaker)

	var command string
	var args []string
	if len(os.Args) > 0 {
		command, args = os.Args[1], os.Args[2:]
	}

	err := commandSet.Execute(command, args)
	if err != nil {
		log.Fatal(err)
	}
}
