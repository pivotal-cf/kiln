package baking

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/pivotal-cf/kiln/internal/builder"
)

type ReleasesService struct {
	logger logger
	reader partReader
}

func NewReleasesService(logger logger, reader partReader) ReleasesService {
	return ReleasesService{
		logger: logger,
		reader: reader,
	}
}

func (s ReleasesService) FromDirectories(directories []string) (map[string]interface{}, error) {
	s.logger.Println("Reading release manifests...")

	var releases []builder.Part
	for _, directory := range directories {
		newReleases, err := s.ReleasesInDirectory(directory)
		if err != nil {
			return nil, err
		}

		releases = append(releases, newReleases...)
	}

	manifests := map[string]interface{}{}
	for _, rel := range releases {
		manifests[rel.Name] = rel.Metadata
	}

	return manifests, nil
}

func (s ReleasesService) ReleasesInDirectory(directoryPath string) ([]builder.Part, error) {
	var tarballPaths []string

	err := filepath.Walk(directoryPath, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if match, _ := regexp.MatchString("tgz$|tar.gz$", path); match {
			tarballPaths = append(tarballPaths, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	var releases []builder.Part
	for _, tarballPath := range tarballPaths {
		rel, err := s.reader.Read(tarballPath)
		if err != nil {
			return nil, err
		}

		releases = append(releases, rel)
	}

	return releases, err
}
