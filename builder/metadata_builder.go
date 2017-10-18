package builder

import (
	"fmt"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

type MetadataBuilder struct {
	releaseManifestReader  releaseManifestReader
	stemcellManifestReader stemcellManifestReader
	handcraftReader        handcraftReader
	logger                 logger
}

type Metadata struct {
	Name             string
	Releases         []MetadataRelease
	StemcellCriteria MetadataStemcellCriteria `yaml:"stemcell_criteria"`
	Handcraft        Handcraft                `yaml:",inline"`
}

type MetadataRelease struct {
	Name    string
	File    string
	Version string
}

type MetadataStemcellCriteria struct {
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

type handcraftReader interface {
	Read(path, version string) (Handcraft, error)
}

type logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

func NewMetadataBuilder(releaseManifestReader releaseManifestReader, stemcellManifestReader stemcellManifestReader, handcraftReader handcraftReader, logger logger) MetadataBuilder {
	return MetadataBuilder{
		releaseManifestReader:  releaseManifestReader,
		stemcellManifestReader: stemcellManifestReader,
		handcraftReader:        handcraftReader,
		logger:                 logger,
	}
}

func (m MetadataBuilder) Build(releaseTarballs []string, pathToStemcell, pathToHandcraft, name, version, pathToTile string) (Metadata, error) {
	m.logger.Printf("Creating metadata for %s...", pathToTile)

	var releases []MetadataRelease
	for _, releaseTarball := range releaseTarballs {
		releaseManifest, err := m.releaseManifestReader.Read(releaseTarball)
		if err != nil {
			return Metadata{}, err
		}

		m.logger.Printf("Read manifest for release %s", releaseManifest.Name)

		releases = append(releases, MetadataRelease{
			Name:    releaseManifest.Name,
			Version: releaseManifest.Version,
			File:    filepath.Base(releaseTarball),
		})
	}

	stemcellManifest, err := m.stemcellManifestReader.Read(pathToStemcell)

	if err != nil {
		return Metadata{}, err
	}

	m.logger.Printf("Read manifest for stemcell version %s", stemcellManifest.Version)

	handcraft, err := m.handcraftReader.Read(pathToHandcraft, version)
	if err != nil {
		return Metadata{}, err
	}

	m.logger.Printf("Read metadata")

	handcraft, err = m.updateRuntimeConfigReleaseVersions(handcraft, releases)
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		Name:     name,
		Releases: releases,
		StemcellCriteria: MetadataStemcellCriteria{
			OS:          stemcellManifest.OperatingSystem,
			Version:     stemcellManifest.Version,
			RequiresCPI: false,
		},
		Handcraft: handcraft,
	}, nil
}

func (m MetadataBuilder) updateRuntimeConfigReleaseVersions(handcraft Handcraft, releases []MetadataRelease) (Handcraft, error) {
	if opsmanRuntimeConfigs, ok := handcraft["runtime_configs"]; ok {
		for _, orc := range opsmanRuntimeConfigs.([]interface{}) {
			opsmanRuntimeConfig := orc.(map[interface{}]interface{})
			var boshRuntimeConfig BoshRuntimeConfig
			err := yaml.Unmarshal([]byte(opsmanRuntimeConfig["runtime_config"].(string)), &boshRuntimeConfig)
			if err != nil {
				return Handcraft{}, fmt.Errorf("runtime config %s contains malformed yaml: %s",
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
						return Handcraft{}, fmt.Errorf("runtime config %s references unknown release %s",
							opsmanRuntimeConfig["name"], runtimeConfigRelease["name"])
					}
				}
			}

			newYAML, err := yaml.Marshal(boshRuntimeConfig)
			if err != nil {
				return Handcraft{}, err // untested
			}

			opsmanRuntimeConfig["runtime_config"] = string(newYAML)
		}
	}

	return handcraft, nil
}
