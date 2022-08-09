package baking

import "os"

type MetadataService struct{}

func NewMetadataService() MetadataService {
	return MetadataService{}
}

func (ms MetadataService) Read(path string) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return contents, nil
}
