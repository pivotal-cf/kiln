package kiln

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	yaml "gopkg.in/yaml.v2"
)

//go:generate counterfeiter -o ./fakes/tile_writer.go --fake-name TileWriter . tileWriter
type tileWriter interface {
	Write(metadataContents []byte, config commands.BakeConfig) error
}

//go:generate counterfeiter -o ./fakes/metadata_builder.go --fake-name MetadataBuilder . metadataBuilder
type metadataBuilder interface {
	Build(releaseTarballs []string, pathToStemcell, pathToHandcraft, name, version, pathToTile string) (builder.Metadata, error)
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

func (t TileMaker) Make(config commands.BakeConfig) error {
	files, err := ioutil.ReadDir(config.ReleasesDirectory)
	if err != nil {
		return err
	}

	var releaseTarballs []string
	for _, file := range files {
		releaseTarballs = append(releaseTarballs, filepath.Join(config.ReleasesDirectory, file.Name()))
	}

	metadata, err := t.metadataBuilder.Build(
		releaseTarballs,
		config.StemcellTarball,
		config.Handcraft,
		config.ProductName,
		config.Version,
		config.OutputFile,
	)
	if err != nil {
		return err
	}

	t.logger.Println("Marshaling metadata file...")
	metadataYAML, err := yaml.Marshal(metadata)
	if err != nil {
		return err
	}

	err = t.tileWriter.Write(metadataYAML, config)
	if err != nil {
		return err
	}

	return nil

}
