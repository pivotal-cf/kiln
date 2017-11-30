package kiln

import (
	"io/ioutil"
	"path/filepath"
	"regexp"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	yaml "gopkg.in/yaml.v2"
)

//go:generate counterfeiter -o ./fakes/tile_writer.go --fake-name TileWriter . tileWriter

type tileWriter interface {
	Write(productName string, generatedMetadataContents []byte, config commands.BakeConfig) error
}

//go:generate counterfeiter -o ./fakes/metadata_builder.go --fake-name MetadataBuilder . metadataBuilder

type metadataBuilder interface {
	Build(input builder.BuildInput) (builder.GeneratedMetadata, error)
}

type logger interface {
	Printf(format string, v ...interface{})
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
	var releaseTarballs []string
	for _, releasesDirectory := range config.ReleaseDirectories {
		files, err := ioutil.ReadDir(releasesDirectory)
		if err != nil {
			return err
		}

		for _, file := range files {
			matchTarballs, _ := regexp.MatchString("tgz$|tar.gz$", file.Name())
			if !matchTarballs {
				continue
			}
			releaseTarballs = append(releaseTarballs, filepath.Join(releasesDirectory, file.Name()))
		}
	}

	t.logger.Printf("Creating metadata for %s...", config.OutputFile)

	buildInput := builder.BuildInput{
		MetadataPath:             config.Metadata,
		ReleaseTarballs:          releaseTarballs,
		StemcellTarball:          config.StemcellTarball,
		FormDirectories:          config.FormDirectories,
		InstanceGroupDirectories: config.InstanceGroupDirectories,
		JobDirectories:           config.JobDirectories,
		RuntimeConfigDirectories: config.RuntimeConfigDirectories,
		VariableDirectories:      config.VariableDirectories,
		IconPath:                 config.IconPath,
		Version:                  config.Version,
	}

	generatedMetadata, err := t.metadataBuilder.Build(buildInput)
	if err != nil {
		return err
	}

	t.logger.Println("Marshaling metadata file...")

	generatedMetadataYAML, err := yaml.Marshal(generatedMetadata)
	if err != nil {
		return err
	}

	err = t.tileWriter.Write(generatedMetadata.Name, generatedMetadataYAML, config)
	if err != nil {
		return err
	}

	return nil
}
