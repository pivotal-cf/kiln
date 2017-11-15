package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

type MetadataPartsDirectoryReader struct {
	filesystem  filesystem
	topLevelKey string
}

func NewMetadataPartsDirectoryReader(filesystem filesystem, topLevelKey string) MetadataPartsDirectoryReader {
	return MetadataPartsDirectoryReader{filesystem: filesystem, topLevelKey: topLevelKey}
}

func (r MetadataPartsDirectoryReader) Read(path string) ([]interface{}, error) {
	parts := []interface{}{}
	err := r.filesystem.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(filePath) != ".yml" {
			return nil
		}
		f, err := r.filesystem.Open(filePath)
		if err != nil {
			return err
		}
		defer f.Close()
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		var fileVars map[string][]interface{}
		err = yaml.Unmarshal([]byte(data), &fileVars)
		if err != nil {
			return err
		}
		_, ok := fileVars[r.topLevelKey]
		if !ok {
			return fmt.Errorf("not a %s file: %q", r.topLevelKey, filePath)
		}
		parts = append(parts, fileVars[r.topLevelKey]...)
		return nil
	})
	if err != nil {
		return []interface{}{}, err
	}
	return parts, nil
}
