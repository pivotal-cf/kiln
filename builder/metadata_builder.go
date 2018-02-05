package builder

import (
	"fmt"
)

type MetadataBuilder struct {
	instanceGroupDirectoryReader metadataPartsDirectoryReader
	jobsDirectoryReader          metadataPartsDirectoryReader
	iconEncoder                  iconEncoder
	logger                       logger
	metadataReader               metadataReader
	variablesDirectoryReader     metadataPartsDirectoryReader
}

type BuildInput struct {
	IconPath                string
	MetadataPath            string
	BOSHVariableDirectories []string
	Version                 string
}

//go:generate counterfeiter -o ./fakes/metadata_parts_directory_reader.go --fake-name MetadataPartsDirectoryReader . metadataPartsDirectoryReader

type metadataPartsDirectoryReader interface {
	Read(path string) ([]Part, error)
}

//go:generate counterfeiter -o ./fakes/metadata_reader.go --fake-name MetadataReader . metadataReader

type metadataReader interface {
	Read(path, version string) (Metadata, error)
}

//go:generate counterfeiter -o ./fakes/icon_encoder.go --fake-name IconEncoder . iconEncoder

type iconEncoder interface {
	Encode(path string) (string, error)
}

type logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

func NewMetadataBuilder(
	variablesDirectoryReader metadataPartsDirectoryReader,
	metadataReader metadataReader,
	logger logger,
	iconEncoder iconEncoder,
) MetadataBuilder {
	return MetadataBuilder{
		iconEncoder:              iconEncoder,
		logger:                   logger,
		metadataReader:           metadataReader,
		variablesDirectoryReader: variablesDirectoryReader,
	}
}

func (m MetadataBuilder) Build(input BuildInput) (GeneratedMetadata, error) {
	metadata, err := m.metadataReader.Read(input.MetadataPath, input.Version)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	variables, err := m.buildVariables(input.BOSHVariableDirectories, metadata)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	encodedIcon := ""
	if input.IconPath != "" {
		encodedIcon, err = m.iconEncoder.Encode(input.IconPath)
		if err != nil {
			return GeneratedMetadata{}, err
		}
	}

	return GeneratedMetadata{
		IconImage: encodedIcon,
		Variables: variables,
		Metadata:  metadata,
	}, nil
}

func (m MetadataBuilder) buildVariables(vars []string, metadata Metadata) ([]Part, error) {
	var variables []Part

	for _, variablesDirectory := range vars {
		m.logger.Printf("Reading variables from %s", variablesDirectory)

		v, err := m.variablesDirectoryReader.Read(variablesDirectory)
		if err != nil {
			return nil,
				fmt.Errorf("error reading from variables directory %q: %s", variablesDirectory, err)
		}

		variables = append(variables, v...)
	}

	return variables, nil
}
