package builder

import (
	"errors"
	"path/filepath"
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

//go:generate counterfeiter -o ./fakes/stemcell_manifest_reader.go --fake-name StemcellManifestReader . stemcellManifestReader
type stemcellManifestReader interface {
	Read(path string) (StemcellManifest, error)
}

//go:generate counterfeiter -o ./fakes/metadata_reader.go --fake-name MetadataReader . metadataReader
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

func (m MetadataBuilder) Build(releaseTarballs []string, pathToStemcell, pathToMetadata, version, pathToTile string) (GeneratedMetadata, error) {
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

	productName, ok := metadata["name"].(string)
	if !ok {
		return GeneratedMetadata{}, errors.New(`missing "name" in tile metadata`)
	}

	delete(metadata, "name")

	m.logger.Printf("Read metadata")

	return GeneratedMetadata{
		Name:     productName,
		Releases: releases,
		StemcellCriteria: StemcellCriteria{
			OS:          stemcellManifest.OperatingSystem,
			Version:     stemcellManifest.Version,
			RequiresCPI: false,
		},
		Metadata: metadata,
	}, nil
}
