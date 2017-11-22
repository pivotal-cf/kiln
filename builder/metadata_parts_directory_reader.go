package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	pathPkg "path"

	yaml "gopkg.in/yaml.v2"
)

type MetadataPartsDirectoryReader struct {
	filesystem  filesystem
	topLevelKey string
	orderKey    string
}

func NewMetadataPartsDirectoryReader(filesystem filesystem, topLevelKey string) MetadataPartsDirectoryReader {
	return MetadataPartsDirectoryReader{filesystem: filesystem, topLevelKey: topLevelKey}
}

func NewMetadataPartsDirectoryReaderWithOrder(filesystem filesystem, topLevelKey, orderKey string) MetadataPartsDirectoryReader {
	return MetadataPartsDirectoryReader{filesystem: filesystem, topLevelKey: topLevelKey, orderKey: orderKey}
}

func (r MetadataPartsDirectoryReader) Read(path string) ([]interface{}, error) {
	metadataFileContents, err := r.readMetadataRecursivelyFromDir(path)
	if err != nil {
		return []interface{}{}, err
	}

	if r.orderKey != "" {
		return r.orderWithOrderFile(path, metadataFileContents)
	}

	return r.orderAlphabeticallyByName(path, metadataFileContents)
}

func (r MetadataPartsDirectoryReader) readMetadataRecursivelyFromDir(path string) (map[interface{}]interface{}, error) {
	parts := map[interface{}]interface{}{}

	err := r.filesystem.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(filePath) != ".yml" || pathPkg.Base(filePath) == "_order.yml" {
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

		var fileVars map[string]interface{}
		err = yaml.Unmarshal([]byte(data), &fileVars)
		if err != nil {
			return err
		}

		vars, ok := fileVars[r.topLevelKey]
		if !ok {
			return fmt.Errorf("not a %s file: %q", r.topLevelKey, filePath)
		}

		err = r.readMetadataIntoParts(vars, parts)
		if err != nil {
			return fmt.Errorf("file '%s' with top-level key '%s' has an invalid format: %s", filePath, r.topLevelKey, err)
		}

		return nil
	})

	return parts, err
}

func (r MetadataPartsDirectoryReader) readMetadataIntoParts(vars interface{}, parts map[interface{}]interface{}) error {
	switch v := vars.(type) {
	case []interface{}:
		for _, item := range v {
			i, ok := item.(map[interface{}]interface{})
			if !ok {
				return fmt.Errorf("metadata item '%v' must be a map", item)
			}
			name, ok := i["name"]
			if !ok {
				return fmt.Errorf("metadata item '%v' does not have a `name` field", item)
			}
			parts[name] = item
		}
	case map[interface{}]interface{}:
		name, ok := v["name"]
		if !ok {
			return fmt.Errorf("metadata item '%v' does not have a `name` field", v)
		}
		parts[name] = v
	default:
		return fmt.Errorf("expected either slice or map value")
	}

	return nil
}

func (r MetadataPartsDirectoryReader) orderWithOrderFile(path string, parts map[interface{}]interface{}) ([]interface{}, error) {
	orderPath := filepath.Join(path, "_order.yml")
	f, err := r.filesystem.Open(orderPath)
	if err != nil {
		return []interface{}{}, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return []interface{}{}, err
	}

	var files map[string][]interface{}
	err = yaml.Unmarshal([]byte(data), &files)
	if err != nil {
		return []interface{}{}, fmt.Errorf("Invalid format for '%s': %s", orderPath, err)
	}

	orderedNames, ok := files[r.orderKey]
	if !ok {
		return []interface{}{}, fmt.Errorf("Could not find top-level order key '%s' in '%s'", r.orderKey, orderPath)
	}

	var outputs []interface{}
	for _, name := range orderedNames {
		v, ok := parts[name]
		if !ok {
			return []interface{}{}, fmt.Errorf("could not find metadata with `name` '%s' in list: %v", name, parts)
		}
		outputs = append(outputs, v)
	}

	return outputs, err
}

func (r MetadataPartsDirectoryReader) orderAlphabeticallyByName(path string, parts map[interface{}]interface{}) ([]interface{}, error) {
	var orderedKeys []string
	for name := range parts {
		n, ok := name.(string)
		if !ok {
			return []interface{}{}, fmt.Errorf("expected `name` to be of type string: '%v'", name)
		}
		orderedKeys = append(orderedKeys, n)
	}
	sort.Strings(orderedKeys)

	var outputs []interface{}
	for _, name := range orderedKeys {
		outputs = append(outputs, parts[name])
	}

	return outputs, nil
}
