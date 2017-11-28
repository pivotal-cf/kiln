package builder

import (
	"fmt"
	"path/filepath"
)

type MetadataBuilder struct {
	formDirectoryReader           formDirectoryReader
	iconEncoder                   iconEncoder
	logger                        logger
	metadataReader                metadataReader
	releaseManifestReader         releaseManifestReader
	runtimeConfigsDirectoryReader metadataPartsDirectoryReader
	stemcellManifestReader        stemcellManifestReader
	variablesDirectoryReader      metadataPartsDirectoryReader
}

type GeneratedMetadata struct {
	FormTypes        []interface{} `yaml:"form_types,omitempty"`
	IconImage        string        `yaml:"icon_image"`
	Metadata         Metadata      `yaml:",inline"`
	Name             string
	Releases         []Release
	RuntimeConfigs   []interface{}    `yaml:"runtime_configs,omitempty"`
	StemcellCriteria StemcellCriteria `yaml:"stemcell_criteria"`
	Variables        []interface{}    `yaml:",omitempty"`
}

type Release struct {
	Name    string
	File    string
	Version string
}

type StemcellCriteria struct {
	Version     string
	OS          string
	RequiresCPI bool `yaml:"requires_cpi"`
}

type BoshRuntimeConfigFields map[string]interface{}

type BoshRuntimeConfig struct {
	Releases    []map[string]string     `yaml:",omitempty"`
	OtherFields BoshRuntimeConfigFields `yaml:",inline"`
}

//go:generate counterfeiter -o ./fakes/release_manifest_reader.go --fake-name ReleaseManifestReader . releaseManifestReader

type releaseManifestReader interface {
	Read(path string) (ReleaseManifest, error)
}

//go:generate counterfeiter -o ./fakes/stemcell_manifest_reader.go --fake-name StemcellManifestReader . stemcellManifestReader

type stemcellManifestReader interface {
	Read(path string) (StemcellManifest, error)
}

//go:generate counterfeiter -o ./fakes/metadata_parts_directory_reader.go --fake-name MetadataPartsDirectoryReader . metadataPartsDirectoryReader

type metadataPartsDirectoryReader interface {
	Read(path string) ([]interface{}, error)
}

//go:generate counterfeiter -o ./fakes/form_directory_reader.go --fake-name FormDirectoryReader . formDirectoryReader

type formDirectoryReader interface {
	Read(path string) ([]interface{}, error)
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
	formDirectoryReader formDirectoryReader,
	releaseManifestReader releaseManifestReader,
	runtimeConfigsDirectoryReader,
	variablesDirectoryReader metadataPartsDirectoryReader,
	stemcellManifestReader stemcellManifestReader,
	metadataReader metadataReader,
	logger logger,
	iconEncoder iconEncoder,
) MetadataBuilder {
	return MetadataBuilder{
		formDirectoryReader:           formDirectoryReader,
		iconEncoder:                   iconEncoder,
		logger:                        logger,
		metadataReader:                metadataReader,
		releaseManifestReader:         releaseManifestReader,
		runtimeConfigsDirectoryReader: runtimeConfigsDirectoryReader,
		stemcellManifestReader:        stemcellManifestReader,
		variablesDirectoryReader:      variablesDirectoryReader,
	}
}

func (m MetadataBuilder) Build(
	releaseTarballs,
	runtimeConfigDirectories,
	variableDirectories []string,
	pathToStemcell,
	pathToMetadata,
	version,
	pathToTile,
	pathToIcon string,
	formDirectories []string,
) (GeneratedMetadata, error) {
	m.logger.Printf("Creating metadata for %s...", pathToTile)

	var releases []Release
	for _, releaseTarball := range releaseTarballs {
		releaseManifest, err := m.releaseManifestReader.Read(releaseTarball)
		if err != nil {
			return GeneratedMetadata{}, err
		}

		m.logger.Printf("Read manifest for release %s", releaseManifest.Name)

		releases = append(releases, Release{
			Name:    releaseManifest.Name,
			Version: releaseManifest.Version,
			File:    filepath.Base(releaseTarball),
		})
	}

	var runtimeConfigs []interface{}
	for _, runtimeConfigsDirectory := range runtimeConfigDirectories {
		r, err := m.runtimeConfigsDirectoryReader.Read(runtimeConfigsDirectory)
		if err != nil {
			return GeneratedMetadata{},
				fmt.Errorf("error reading from runtime configs directory %q: %s", runtimeConfigsDirectory, err)
		}

		m.logger.Printf("Read runtime configs from %s", runtimeConfigsDirectory)

		runtimeConfigs = append(runtimeConfigs, r...)
	}

	var variables []interface{}
	for _, variablesDirectory := range variableDirectories {
		v, err := m.variablesDirectoryReader.Read(variablesDirectory)
		if err != nil {
			return GeneratedMetadata{},
				fmt.Errorf("error reading from variables directory %q: %s", variablesDirectory, err)
		}

		m.logger.Printf("Read variables from %s", variablesDirectory)

		variables = append(variables, v...)
	}

	stemcellManifest, err := m.stemcellManifestReader.Read(pathToStemcell)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	m.logger.Printf("Read manifest for stemcell version %s", stemcellManifest.Version)

	encodedIcon, err := m.iconEncoder.Encode(pathToIcon)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	metadata, err := m.metadataReader.Read(pathToMetadata, version)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	productName, ok := metadata["name"].(string)
	if !ok {
		return GeneratedMetadata{}, fmt.Errorf(`missing "name" in tile metadata file '%s'`, pathToMetadata)
	}

	var formTypes []interface{}
	if len(formDirectories) > 0 {
		for _, fd := range formDirectories {
			formTypesInDir, err := m.formDirectoryReader.Read(fd)
			if err != nil {
				return GeneratedMetadata{},
					fmt.Errorf("error reading from form directory %q: %s", fd, err)
			}
			formTypes = append(formTypes, formTypesInDir...)
		}
	} else {
		if ft, ok := metadata["form_types"].([]interface{}); ok {
			formTypes = ft
		}
	}

	delete(metadata, "name")
	delete(metadata, "icon_image")
	delete(metadata, "form_types")

	if _, present := metadata["runtime_configs"]; present {
		return GeneratedMetadata{}, fmt.Errorf("runtime_config section must be defined using --runtime-configs-directory flag, not in %q", pathToMetadata)
	}

	if _, present := metadata["variables"]; present {
		return GeneratedMetadata{}, fmt.Errorf("variables section must be defined using --variables-directory flag, not in %q", pathToMetadata)
	}

	m.logger.Printf("Read metadata")

	return GeneratedMetadata{
		FormTypes:      formTypes,
		IconImage:      encodedIcon,
		Name:           productName,
		Releases:       releases,
		RuntimeConfigs: runtimeConfigs,
		Variables:      variables,
		StemcellCriteria: StemcellCriteria{
			OS:          stemcellManifest.OperatingSystem,
			Version:     stemcellManifest.Version,
			RequiresCPI: false,
		},
		Metadata: metadata,
	}, nil
}
