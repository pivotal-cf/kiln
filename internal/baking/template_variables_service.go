package baking

import (
	"fmt"
	"strings"

	"gopkg.in/src-d/go-billy.v4"

	"gopkg.in/yaml.v2"
)

type TemplateVariablesService struct {
	filesystem billy.Filesystem
}

func NewTemplateVariablesService(fs billy.Filesystem) TemplateVariablesService {
	return TemplateVariablesService{filesystem: fs}
}

func (s TemplateVariablesService) FromPathsAndPairs(paths []string, pairs []string) (map[string]interface{}, error) {
	variables := map[string]interface{}{}

	for _, path := range paths {
		file, err := s.filesystem.Open(path)
		if err != nil {
			return nil, fmt.Errorf("unable to open file %q: %w", path, err)
		}

		err = yaml.NewDecoder(file).Decode(&variables)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("unable to YAML parse %q: %w", path, err)
		}
	}

	for _, pair := range pairs {
		parts := strings.Split(pair, "=")

		if len(parts) < 2 {
			return nil, fmt.Errorf("could not parse variable %q: expected variable in \"key=value\" form", pair)
		}

		variables[parts[0]] = parts[1]
	}

	return variables, nil
}
