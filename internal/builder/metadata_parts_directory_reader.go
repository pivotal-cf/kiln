package builder

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v2"
)

type MetadataPartsDirectoryReader struct {
	topLevelKey string
	orderKey    string
}

type Part struct {
	File     string
	Name     string
	Metadata any
}

func NewMetadataPartsDirectoryReader() MetadataPartsDirectoryReader {
	return MetadataPartsDirectoryReader{}
}

func NewMetadataPartsDirectoryReaderWithTopLevelKey(topLevelKey string) MetadataPartsDirectoryReader {
	return MetadataPartsDirectoryReader{topLevelKey: topLevelKey}
}

func NewMetadataPartsDirectoryReaderWithOrder(topLevelKey, orderKey string) MetadataPartsDirectoryReader {
	return MetadataPartsDirectoryReader{topLevelKey: topLevelKey, orderKey: orderKey}
}

func (r MetadataPartsDirectoryReader) Read(path string) ([]Part, error) {
	parts, err := r.readMetadataRecursivelyFromDir(path, nil)
	if err != nil {
		return []Part{}, err
	}

	if r.orderKey != "" {
		return r.orderWithOrderFromFile(path, parts)
	}

	return r.orderAlphabeticallyByName(path, parts)
}

func (r MetadataPartsDirectoryReader) ParseMetadataTemplates(directories []string, variables map[string]any) (map[string]any, error) {
	var releases []Part
	for _, directory := range directories {
		newReleases, err := r.ReadPreProcess(directory, variables)
		if err != nil {
			return nil, err
		}

		releases = append(releases, newReleases...)
	}

	manifests := map[string]any{}
	for _, rel := range releases {
		manifests[rel.Name] = rel.Metadata
	}

	return manifests, nil
}

func (r MetadataPartsDirectoryReader) ReadPreProcess(path string, variables map[string]any) ([]Part, error) {
	parts, err := r.readMetadataRecursivelyFromDir(path, variables)
	if err != nil {
		return []Part{}, err
	}

	if r.orderKey != "" {
		return r.orderWithOrderFromFile(path, parts)
	}

	return r.orderAlphabeticallyByName(path, parts)
}

func (r MetadataPartsDirectoryReader) readMetadataRecursivelyFromDir(p string, variables map[string]any) ([]Part, error) {
	var parts []Part

	var buf bytes.Buffer

	err := filepath.Walk(p, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(filePath) != ".yml" || path.Base(filePath) == "_order.yml" {
			return nil
		}
		defer buf.Reset()

		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		if variables != nil {
			err = PreProcessMetadataWithTileFunction(variables, p, &buf, data)
			if err != nil {
				return err
			}
			data = buf.Bytes()
		}

		var vars any
		if r.topLevelKey != "" {
			var fileVars map[string]any
			err = yaml.Unmarshal(data, &fileVars)
			if err != nil {
				return fmt.Errorf("cannot unmarshal '%s': %w", filePath, err)
			}

			var ok bool
			vars, ok = fileVars[r.topLevelKey]
			if !ok {
				return fmt.Errorf("not a %s file: %q", r.topLevelKey, filePath)
			}
		} else {
			err = yaml.Unmarshal(data, &vars)
			if err != nil {
				return fmt.Errorf("cannot unmarshal '%s': %w", filePath, err)
			}
		}

		parts, err = r.readMetadataIntoParts(path.Base(filePath), vars, parts)
		if err != nil {
			return fmt.Errorf("file '%s' with top-level key '%s' has an invalid format: %w", filePath, r.topLevelKey, err)
		}

		return nil
	})

	return parts, err
}

func (r MetadataPartsDirectoryReader) readMetadataIntoParts(fileName string, vars any, parts []Part) ([]Part, error) {
	switch v := vars.(type) {
	case []any:
		for _, item := range v {
			i, ok := item.(map[any]any)
			if !ok {
				return []Part{}, fmt.Errorf("metadata item '%v' must be a map", item)
			}

			part, err := r.buildPartFromMetadata(i, fileName)
			if err != nil {
				return []Part{}, err
			}

			parts = append(parts, part)
		}
	case map[any]any:
		part, err := r.buildPartFromMetadata(v, fileName)
		if err != nil {
			return []Part{}, err
		}
		parts = append(parts, part)
	default:
		return []Part{}, fmt.Errorf("expected either slice or map value")
	}

	return parts, nil
}

func (r MetadataPartsDirectoryReader) buildPartFromMetadata(metadata map[any]any, legacyFilename string) (Part, error) {
	name, ok := metadata["alias"].(string)
	if !ok {
		name, ok = metadata["name"].(string)
		if !ok {
			return Part{}, fmt.Errorf("metadata item '%v' does not have a `name` field", metadata)
		}
	}
	delete(metadata, "alias")

	return Part{File: legacyFilename, Name: name, Metadata: metadata}, nil
}

func (r MetadataPartsDirectoryReader) orderWithOrderFromFile(path string, parts []Part) ([]Part, error) {
	orderPath := filepath.Join(path, "_order.yml")
	f, err := os.Open(orderPath)
	if err != nil {
		return []Part{}, err
	}
	defer closeAndIgnoreError(f)

	data, err := io.ReadAll(f)
	if err != nil {
		return []Part{}, err
	}

	var files map[string][]any
	err = yaml.Unmarshal(data, &files)
	if err != nil {
		return []Part{}, fmt.Errorf("invalid format for %q: %w", orderPath, err)
	}

	orderedNames, ok := files[r.orderKey]
	if !ok {
		return []Part{}, fmt.Errorf("could not find top-level order key %q in %q", r.orderKey, orderPath)
	}

	var outputs []Part
	for _, name := range orderedNames {
		found := false
		for _, part := range parts {
			if part.Name == name {
				found = true
				outputs = append(outputs, part)
			}
		}
		if !found {
			return []Part{}, fmt.Errorf("file specified in _order.yml %q does not exist in %q", name, path)
		}
	}

	return outputs, err
}

func (r MetadataPartsDirectoryReader) orderAlphabeticallyByName(_ string, parts []Part) ([]Part, error) {
	var orderedKeys []string
	for _, part := range parts {
		orderedKeys = append(orderedKeys, part.Name)
	}
	sort.Strings(orderedKeys)

	var outputs []Part
	for _, name := range orderedKeys {
		for _, part := range parts {
			if part.Name == name {
				outputs = append(outputs, part)
			}
		}
	}

	return outputs, nil
}
