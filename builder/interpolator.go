package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"text/template"

	yaml "gopkg.in/yaml.v2"
)

type Interpolator struct{}

type InterpolateInput struct {
	Variables        map[string]string
	ReleaseManifests map[string]ReleaseManifest
	StemcellManifest StemcellManifest
	FormTypes        map[string]interface{}
}

func NewInterpolator() Interpolator {
	return Interpolator{}
}

func (i Interpolator) Interpolate(input InterpolateInput, templateYAML []byte) ([]byte, error) {
	interpolatedYAML, err := i.interpolate(input, templateYAML)
	if err != nil {
		return nil, err
	}

	prettyMetadata, err := i.prettyPrint(interpolatedYAML)
	if err != nil {
		return nil, err // un-tested
	}

	return prettyMetadata, nil
}

func (i Interpolator) interpolate(input InterpolateInput, templateYAML []byte) ([]byte, error) {
	templateHelpers := template.FuncMap{
		"form": func(key string) (string, error) {
			val, ok := input.FormTypes[key]
			if !ok {
				return "", fmt.Errorf("could not find form with key '%s'", key)
			}

			return i.interpolateValueIntoYAML(input, val)
		},
		"release": func(name string) (string, error) {
			val, ok := input.ReleaseManifests[name]
			if !ok {
				return "", fmt.Errorf("could not find release with name '%s'", name)
			}

			return i.interpolateValueIntoYAML(input, val)
		},
		"stemcell": func() (string, error) {
			if input.StemcellManifest == (StemcellManifest{}) {
				return "", errors.New("stemcell-tarball must be specified")
			}
			return i.interpolateValueIntoYAML(input, input.StemcellManifest)
		},
		"variable": func(key string) (string, error) {
			val, ok := input.Variables[key]
			if !ok {
				return "", fmt.Errorf("could not find variable with key '%s'", key)
			}
			return val, nil
		},
	}

	t, err := template.New("metadata").
		Delims("$(", ")").
		Funcs(templateHelpers).
		Parse(string(templateYAML))

	if err != nil {
		return nil, fmt.Errorf("template parsing failed: %s", err)
	}

	var buffer bytes.Buffer
	err = t.Execute(&buffer, input.Variables)
	if err != nil {
		return nil, fmt.Errorf("template execution failed: %s", err)
	}

	return buffer.Bytes(), nil
}

func (i Interpolator) interpolateValueIntoYAML(input InterpolateInput, val interface{}) (string, error) {
	initialYAML, err := yaml.Marshal(val)
	if err != nil {
		return "", err // should never happen
	}

	interpolatedYAML, err := i.interpolate(input, initialYAML)
	if err != nil {
		return "", fmt.Errorf("unable to interpolate value: %s", err)
	}

	inlinedYAML, err := i.yamlMarshalOneLine(interpolatedYAML)
	if err != nil {
		return "", err // un-tested
	}

	return string(inlinedYAML), nil
}

// Workaround to avoid YAML indentation being incorrect when value is interpolated into the metadata
func (i Interpolator) yamlMarshalOneLine(yamlContents []byte) ([]byte, error) {
	contents := map[string]interface{}{}
	err := yaml.Unmarshal(yamlContents, &contents)
	if err != nil {
		return nil, err
	}

	return json.Marshal(contents)
}

func (i Interpolator) prettyPrint(inputYAML []byte) ([]byte, error) {
	var data map[string]interface{}
	err := yaml.Unmarshal(inputYAML, &data)
	if err != nil {
		return []byte{}, err // should never happen
	}

	return yaml.Marshal(data)
}
