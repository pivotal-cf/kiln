package kiln

import (
	"github.com/pivotal-cf/kiln/builder"
	yaml "gopkg.in/yaml.v2"
)

//go:generate counterfeiter -o ./fakes/tile_writer.go --fake-name TileWriter . tileWriter
type tileWriter interface {
	Write(metadataContents []byte, config builder.WriteConfig) error
}

//go:generate counterfeiter -o ./fakes/metadata_builder.go --fake-name MetadataBuilder . metadataBuilder
type metadataBuilder interface {
	Build(releaseTarballs []string, pathToStemcell, pathToHandcraft, name, version string) (builder.Metadata, error)
}

type logger interface {
	Println(v ...interface{})
}

type TileMaker struct {
	metadataBuilder metadataBuilder
	tileWriter      tileWriter
	logger          logger
}

func NewTileMaker(metadataBuilder metadataBuilder, tileWriter tileWriter, logger logger) TileMaker {
	return TileMaker{
		metadataBuilder: metadataBuilder,
		tileWriter:      tileWriter,
		logger:          logger,
	}
}

func (t TileMaker) Make(config ApplicationConfig) error {
	metadata, err := t.metadataBuilder.Build(
		config.ReleaseTarballs,
		config.StemcellTarball,
		config.Handcraft,
		config.ProductName,
		config.FinalVersion,
	)
	if err != nil {
		return err
	}

	t.logger.Println("Marshaling metadata file...")
	metadataYAML, err := yaml.Marshal(metadata)
	if err != nil {
		return err
	}

	err = t.tileWriter.Write(metadataYAML, builder.WriteConfig{
		ReleaseTarballs:      config.ReleaseTarballs,
		MigrationsDirectory:  config.MigrationsDirectory,
		ContentMigrations:    config.ContentMigrations,
		BaseContentMigration: config.BaseContentMigration,
		StemcellTarball:      config.StemcellTarball,
		Handcraft:            config.Handcraft,
		Version:              config.Version,
		FinalVersion:         config.FinalVersion,
		ProductName:          config.ProductName,
		FilenamePrefix:       config.FilenamePrefix,
		OutputDir:            config.OutputDir,
		StubReleases:         config.StubReleases,
	})
	if err != nil {
		return err
	}

	return nil

}
