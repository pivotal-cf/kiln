package main

import (
	"fmt"
	"log"
	"os"

	"github.com/pivotal-cf/kiln/builder"
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
	argParser := kiln.NewArgParser()
	tileMaker := kiln.NewTileMaker(metadataBuilder, tileWriter, logger)

	app := kiln.NewApplication(argParser, tileMaker)
	err := app.Run(os.Args[1:])
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
