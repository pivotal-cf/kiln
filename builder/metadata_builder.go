package builder

import (
	"fmt"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

type MetadataBuilder struct {
	releaseManifestReader  releaseManifestReader
	stemcellManifestReader stemcellManifestReader
	metadataReader         metadataReader
	logger                 logger
}

type GeneratedMetadata struct {
	Name             string
	Releases         []Release
	StemcellCriteria StemcellCriteria `yaml:"stemcell_criteria"`
	Metadata         Metadata         `yaml:",inline"`
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

type releaseManifestReader interface {
	Read(path string) (ReleaseManifest, error)
}

type stemcellManifestReader interface {
	Read(path string) (StemcellManifest, error)
}

type metadataReader interface {
	Read(path, version string) (Metadata, error)
}

type logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

func NewMetadataBuilder(releaseManifestReader releaseManifestReader, stemcellManifestReader stemcellManifestReader, metadataReader metadataReader, logger logger) MetadataBuilder {
	return MetadataBuilder{
		releaseManifestReader:  releaseManifestReader,
		stemcellManifestReader: stemcellManifestReader,
		metadataReader:         metadataReader,
		logger:                 logger,
	}
}

func (m MetadataBuilder) Build(releaseTarballs []string, pathToStemcell, pathToMetadata, name, version, pathToTile string) (GeneratedMetadata, error) {
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

	stemcellManifest, err := m.stemcellManifestReader.Read(pathToStemcell)

	if err != nil {
		return GeneratedMetadata{}, err
	}

	m.logger.Printf("Read manifest for stemcell version %s", stemcellManifest.Version)

	metadata, err := m.metadataReader.Read(pathToMetadata, version)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	m.logger.Printf("Read metadata")

	metadata, err = m.updateRuntimeConfigReleaseVersions(metadata, releases)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	return GeneratedMetadata{
		Name:     name,
		Releases: releases,
		StemcellCriteria: StemcellCriteria{
			OS:          stemcellManifest.OperatingSystem,
			Version:     stemcellManifest.Version,
			RequiresCPI: false,
		},
		Metadata: metadata,
	}, nil
}

func (m MetadataBuilder) updateRuntimeConfigReleaseVersions(metadata Metadata, releases []Release) (Metadata, error) {
	if opsmanRuntimeConfigs, ok := metadata["runtime_configs"]; ok {
		for _, orc := range opsmanRuntimeConfigs.([]interface{}) {
			opsmanRuntimeConfig := orc.(map[interface{}]interface{})
			var boshRuntimeConfig BoshRuntimeConfig
			err := yaml.Unmarshal([]byte(opsmanRuntimeConfig["runtime_config"].(string)), &boshRuntimeConfig)
			if err != nil {
				return Metadata{}, fmt.Errorf("runtime config %s contains malformed yaml: %s",
					opsmanRuntimeConfig["name"], err)
			}

			if len(boshRuntimeConfig.Releases) > 0 {
				for _, runtimeConfigRelease := range boshRuntimeConfig.Releases {
					found := false

					for _, release := range releases {
						if release.Name == runtimeConfigRelease["name"] {
							m.logger.Printf("Injecting version %s into runtime config release %s", release.Version, release.Name)
							runtimeConfigRelease["version"] = release.Version
							found = true
						}
					}

					if !found {
						return Metadata{}, fmt.Errorf("runtime config %s references unknown release %s",
							opsmanRuntimeConfig["name"], runtimeConfigRelease["name"])
					}
				}
			}

			newYAML, err := yaml.Marshal(boshRuntimeConfig)
			if err != nil {
				return Metadata{}, err // untested
			}

			opsmanRuntimeConfig["runtime_config"] = string(newYAML)
		}
	}

	return metadata, nil
}
