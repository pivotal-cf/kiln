package baking

import (
	"fmt"
	"strings"

	"github.com/go-git/go-billy/v5"

	"gopkg.in/yaml.v2"
)

type TemplateVariablesService struct {
	filesystem billy.Basic
}

func NewTemplateVariablesService(fs billy.Basic) TemplateVariablesService {
	return TemplateVariablesService{filesystem: fs}
}

func (s TemplateVariablesService) FromPathsAndPairs(paths []string, pairs []string) (map[string]interface{}, error) {
	variables := map[string]interface{}{}

	for _, path := range paths {
		err := parseVariablesFromFile(s.filesystem, path, variables)
		if err != nil {
			return nil, err
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

func parseVariablesFromFile(fs billy.Basic, path string, variables map[string]interface{}) error {
	file, err := fs.Open(path)
	if err != nil {
		return fmt.Errorf("unable to open file %q: %w", path, err)
	}

	err = yaml.NewDecoder(file).Decode(&variables)
	defer closeAndIgnoreError(file)
	if err != nil {
		return fmt.Errorf("unable to YAML parse %q: %w", path, err)
	}
	return nil
}
