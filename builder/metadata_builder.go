package builder

import "path/filepath"

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

func (m MetadataBuilder) Build(releaseTarballs []string, pathToStemcell, pathToHandcraft, name, version string) (Metadata, error) {
	m.logger.Printf("Creating metadata for .pivotal...")

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

	m.logger.Printf("Read handcraft")

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
