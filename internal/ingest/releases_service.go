package ingest

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/pivotal-cf/kiln/builder"
)

//go:generate counterfeiter -o ./fakes/part_reader.go --fake-name PartReader . partReader
type partReader interface {
	Read(path string) (builder.Part, error)
}

type ReleasesService struct {
	reader partReader
}

func NewReleasesService(reader partReader) ReleasesService {
	return ReleasesService{
		reader: reader,
	}
}

func (s ReleasesService) FromDirectories(directories []string) (map[string]interface{}, error) {
	var tarballs []string
	for _, directory := range directories {
		err := filepath.Walk(directory, filepath.WalkFunc(func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if match, _ := regexp.MatchString("tgz$|tar.gz$", path); match {
				tarballs = append(tarballs, path)
			}

			return nil
		}))

		if err != nil {
			return nil, err
		}
	}

	manifests := map[string]interface{}{}
	for _, tarball := range tarballs {
		manifest, err := s.reader.Read(tarball)
		if err != nil {
			return nil, err
		}

		manifests[manifest.Name] = manifest.Metadata
	}

	return manifests, nil
}
