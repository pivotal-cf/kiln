package builder

import (
	"errors"
	"fmt"
)

type MetadataBuilder struct {
	instanceGroupDirectoryReader  metadataPartsDirectoryReader
	jobsDirectoryReader           metadataPartsDirectoryReader
	iconEncoder                   iconEncoder
	logger                        logger
	metadataReader                metadataReader
	runtimeConfigsDirectoryReader metadataPartsDirectoryReader
	variablesDirectoryReader      metadataPartsDirectoryReader
}

type BuildInput struct {
	IconPath                 string
	MetadataPath             string
	RuntimeConfigDirectories []string
	BOSHVariableDirectories  []string
	Version                  string
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
	runtimeConfigsDirectoryReader,
	variablesDirectoryReader metadataPartsDirectoryReader,
	metadataReader metadataReader,
	logger logger,
	iconEncoder iconEncoder,
) MetadataBuilder {
	return MetadataBuilder{
		iconEncoder:                   iconEncoder,
		logger:                        logger,
		metadataReader:                metadataReader,
		runtimeConfigsDirectoryReader: runtimeConfigsDirectoryReader,
		variablesDirectoryReader:      variablesDirectoryReader,
	}
}

func (m MetadataBuilder) Build(input BuildInput) (GeneratedMetadata, error) {
	metadata, err := m.metadataReader.Read(input.MetadataPath, input.Version)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	productName, ok := metadata["name"].(string)
	if !ok {
		return GeneratedMetadata{}, errors.New("missing \"name\" in tile metadata file")
	}

	runtimeConfigs, err := m.buildRuntimeConfigMetadata(input.RuntimeConfigDirectories, metadata)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	variables, err := m.buildVariables(input.BOSHVariableDirectories, metadata)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	encodedIcon, err := m.iconEncoder.Encode(input.IconPath)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	delete(metadata, "name")

	return GeneratedMetadata{
		IconImage:      encodedIcon,
		Name:           productName,
		RuntimeConfigs: runtimeConfigs,
		Variables:      variables,
		Metadata:       metadata,
	}, nil
}

func (m MetadataBuilder) buildRuntimeConfigMetadata(dirs []string, metadata Metadata) ([]Part, error) {
	if _, ok := metadata["runtime_configs"]; ok {
		return nil, errors.New("runtime_config section must be defined using --runtime-configs-directory flag")
	}

	var runtimeConfigs []Part

	for _, runtimeConfigsDirectory := range dirs {
		m.logger.Printf("Reading runtime configs from %s", runtimeConfigsDirectory)

		r, err := m.runtimeConfigsDirectoryReader.Read(runtimeConfigsDirectory)
		if err != nil {
			return nil,
				fmt.Errorf("error reading from runtime configs directory %q: %s", runtimeConfigsDirectory, err)
		}

		runtimeConfigs = append(runtimeConfigs, r...)
	}

	return runtimeConfigs, nil
}

func (m MetadataBuilder) buildVariables(vars []string, metadata Metadata) ([]Part, error) {
	if _, ok := metadata["variables"]; ok {
		return nil, errors.New("variables section must be defined using --variables-directory flag")
	}

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
