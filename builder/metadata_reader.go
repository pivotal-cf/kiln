package builder

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type MetadataReader struct {
	filesystem filesystem
	logger     logger
}

type Metadata map[string]interface{}

func NewMetadataReader(filesystem filesystem, logger logger) MetadataReader {
	return MetadataReader{
		filesystem: filesystem,
		logger:     logger,
	}
}

func (h MetadataReader) Read(path, version string) (Metadata, error) {
	file, err := h.filesystem.Open(path)
	if err != nil {
		return Metadata{}, err
	}
	defer file.Close()

	contents, err := ioutil.ReadAll(file)
	if err != nil {
		return Metadata{}, err
	}

	var metadata Metadata
	err = yaml.Unmarshal(contents, &metadata)
	if err != nil {
		return Metadata{}, err
	}

	return metadata, nil
}
