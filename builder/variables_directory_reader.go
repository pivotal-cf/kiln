package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

type VariablesSnippet struct{}

type VariablesDirectoryReader struct {
	filesystem filesystem
}

func NewVariablesDirectoryReader(filesystem filesystem) VariablesDirectoryReader {
	return VariablesDirectoryReader{filesystem: filesystem}
}

func (v VariablesDirectoryReader) Read(path string) ([]interface{}, error) {
	vars := []interface{}{}
	err := v.filesystem.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(filePath) != ".yml" {
			return nil
		}
		f, err := v.filesystem.Open(filePath)
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
		_, ok := fileVars["variables"]
		if !ok {
			return fmt.Errorf("not a variables file: %q", filePath)
		}
		vars = append(vars, fileVars["variables"]...)
		return nil
	})
	if err != nil {
		return []interface{}{}, err
	}
	return vars, nil
}
