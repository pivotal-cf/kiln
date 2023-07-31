package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"

	yamlConverter "github.com/ghodss/yaml"
	"gopkg.in/yaml.v2"
)

const (
	// MetadataGitSHAVariable is the name of a special variable computed just in time
	// when referenced. The value is computed by MetadataGitSHA func on InterpolateInput.
	// If the value computed value can be over-written by setting it the variable
	// explicitly like --variable="metadata-git-sha=$(git rev-parse HEAD)".
	MetadataGitSHAVariable = "metadata-git-sha"

	// BuildVersionVariable is the name of a special variable when not set has the value of
	// Version field on InterpolateInput returned.
	BuildVersionVariable = "build-version"

	// TileNameVariable is the name of a special variable that is used as the return variable
	// of the tile function during interpolation. When it is not set, the function returns
	// an error.
	TileNameVariable = "tile_name"
)

type Interpolator struct{}

type InterpolateInput struct {
	Version            string
	KilnVersion        string
	BOSHVariables      map[string]any
	Variables          map[string]any
	ReleaseManifests   map[string]any
	StemcellManifests  map[string]any
	StemcellManifest   any
	FormTypes          map[string]any
	IconImage          string
	InstanceGroups     map[string]any
	Jobs               map[string]any
	PropertyBlueprints map[string]any
	RuntimeConfigs     map[string]any
	StubReleases       bool
	MetadataGitSHA     func() (string, error)
}

func NewInterpolator() Interpolator {
	return Interpolator{}
}

func (i Interpolator) Interpolate(input InterpolateInput, name string, templateYAML []byte) ([]byte, error) {
	var gitMetadataSHA string
	if input.MetadataGitSHA != nil {
		sha, err := input.MetadataGitSHA()
		if err != nil {
			return nil, err
		}
		gitMetadataSHA = sha
	}

	interpolatedYAML, err := i.interpolate(input, name, templateYAML)
	if err != nil {
		return nil, err
	}

	prettyMetadata, err := i.prettyPrint(interpolatedYAML)
	if err != nil {
		return nil, err // un-tested
	}

	return setKilnMetadata(prettyMetadata, KilnMetadata{
		KilnVersion:    input.KilnVersion,
		MetadataGitSHA: gitMetadataSHA,
	})
}

func (i Interpolator) functions(input InterpolateInput) template.FuncMap {
	versionFunc := func() (string, error) {
		if input.Version == "" {
			return "", errors.New("--version must be specified")
		}
		return i.interpolateValueIntoYAML(input, "", input.Version)
	}

	return template.FuncMap{
		"bosh_variable": func(key string) (string, error) {
			if input.BOSHVariables == nil {
				return "", errors.New("--bosh-variables-directory must be specified")
			}
			val, ok := input.BOSHVariables[key]
			if !ok {
				return "", fmt.Errorf("could not find bosh variable with key '%s'", key)
			}
			return i.interpolateValueIntoYAML(input, key, val)
		},
		"form": func(key string) (string, error) {
			if input.FormTypes == nil {
				return "", errors.New("--forms-directory must be specified")
			}
			val, ok := input.FormTypes[key]
			if !ok {
				return "", fmt.Errorf("could not find form with key '%s'", key)
			}

			return i.interpolateValueIntoYAML(input, key, val)
		},
		"property": func(name string) (string, error) {
			if input.PropertyBlueprints == nil {
				return "", errors.New("--properties-directory must be specified")
			}
			val, ok := input.PropertyBlueprints[name]
			if !ok {
				return "", fmt.Errorf("could not find property blueprint with name '%s'", name)
			}
			return i.interpolateValueIntoYAML(input, name, val)
		},
		"regexReplaceAll": func(regex, inputString, replaceString string) (string, error) {
			re, err := regexp.Compile(regex)
			if err != nil {
				return "", err
			}
			return re.ReplaceAllString(inputString, replaceString), nil
		},
		"release": func(name string) (string, error) {
			if input.ReleaseManifests == nil {
				return "", errors.New("missing ReleaseManifests")
			}

			val, ok := input.ReleaseManifests[name]

			if !ok {
				if input.StubReleases {
					val = map[string]any{
						"name":    name,
						"version": "UNKNOWN",
						"file":    fmt.Sprintf("%s-UNKNOWN.tgz", name),
						"sha1":    "dead8e1ea5e00dead8e1ea5ed00ead8e1ea5e000",
					}
				} else {
					return "", fmt.Errorf("could not find release with name '%s'", name)
				}
			}

			return i.interpolateValueIntoYAML(input, name, val)
		},
		"stemcell": func(osname ...string) (string, error) {
			if input.StemcellManifest == nil && len(input.StemcellManifests) == 0 {
				return "", errors.New("stemcell specification must be provided through either --stemcells-directory or --kilnfile")
			}

			if len(input.StemcellManifests) == 0 && len(osname) > 0 {
				return "", errors.New("$( stemcell \"<osname>\" ) cannot be used without --stemcells-directory being provided")
			}

			if len(input.StemcellManifests) > 1 && len(osname) == 0 {
				return "", errors.New("stemcell template helper requires osname argument if multiple stemcells are specified")
			}

			if len(osname) > 0 {
				return i.interpolateValueIntoYAML(input, osname[0], input.StemcellManifests[osname[0]])
			}

			if len(input.StemcellManifests) == 1 {
				for name, stemcell := range input.StemcellManifests {
					return i.interpolateValueIntoYAML(input, name, stemcell)
				}
			}

			return i.interpolateValueIntoYAML(input, "stemcell", input.StemcellManifest)
		},
		"version": versionFunc,
		"variable": func(key string) (string, error) {
			if input.Variables == nil {
				return "", errors.New("--variable or --variables-file must be specified")
			}
			val, ok := input.Variables[key]
			if !ok {
				switch key {
				case MetadataGitSHAVariable:
					if input.MetadataGitSHA != nil {
						return input.MetadataGitSHA()
					}
				case BuildVersionVariable:
					return versionFunc()
				}
				return "", fmt.Errorf("could not find variable with key '%s'", key)
			}
			return i.interpolateValueIntoYAML(input, key, val)
		},
		"icon": func() (string, error) {
			if input.IconImage == "" {
				return "", fmt.Errorf("--icon must be specified")
			}
			return input.IconImage, nil
		},
		"instance_group": func(name string) (string, error) {
			if input.InstanceGroups == nil {
				return "", errors.New("--instance-groups-directory must be specified")
			}
			val, ok := input.InstanceGroups[name]
			if !ok {
				return "", fmt.Errorf("could not find instance_group with name '%s'", name)
			}

			return i.interpolateValueIntoYAML(input, name, val)
		},
		"job": func(name string) (string, error) {
			if input.Jobs == nil {
				return "", errors.New("--jobs-directory must be specified")
			}
			val, ok := input.Jobs[name]
			if !ok {
				return "", fmt.Errorf("could not find job with name '%s'", name)
			}

			return i.interpolateValueIntoYAML(input, name, val)
		},
		"runtime_config": func(name string) (string, error) {
			if input.RuntimeConfigs == nil {
				return "", errors.New("--runtime-configs-directory must be specified")
			}
			val, ok := input.RuntimeConfigs[name]
			if !ok {
				return "", fmt.Errorf("could not find runtime_config with name '%s'", name)
			}

			return i.interpolateValueIntoYAML(input, name, val)
		},
		"select": func(field, input string) (string, error) {
			object := map[string]any{}

			err := json.Unmarshal([]byte(input), &object)
			if err != nil {
				return "", fmt.Errorf("could not JSON unmarshal %q: %s", input, err)
			}

			value, ok := object[field]
			if !ok {
				return "", fmt.Errorf("could not select %q, key does not exist", field)
			}

			output, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("could not JSON marshal %q: %s", input, err) // NOTE: this cannot happen because value was unmarshalled from JSON
			}

			return string(output), nil
		},
		"tile": tileFunc(input.Variables),
	}
}

func (i Interpolator) interpolate(input InterpolateInput, name string, templateYAML []byte) ([]byte, error) {
	t, err := template.New(name).
		Funcs(i.functions(input)).
		Delims("$(", ")").
		Option("missingkey=error").
		Parse(string(templateYAML))
	if err != nil {
		return nil, fmt.Errorf("failed when parsing a %w", err)
	}

	var buffer bytes.Buffer
	err = t.Execute(&buffer, input.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed when rendering a %w", err)
	}

	return buffer.Bytes(), nil
}

func (i Interpolator) interpolateValueIntoYAML(input InterpolateInput, name string, val any) (string, error) {
	initialYAML, err := yaml.Marshal(val)
	if err != nil {
		return "", err // should never happen
	}

	interpolatedYAML, err := i.interpolate(input, name, initialYAML)
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
	return yamlConverter.YAMLToJSON(yamlContents)
}

func (i Interpolator) prettyPrint(inputYAML []byte) ([]byte, error) {
	var data any
	err := yaml.Unmarshal(inputYAML, &data)
	if err != nil {
		return []byte{}, err // should never happen
	}

	return yaml.Marshal(data)
}

func PreProcessMetadataWithTileFunction(variables map[string]any, name string, dst io.Writer, in []byte) error {
	tileFN := tileFunc(variables)

	t, err := template.New(name).
		Funcs(template.FuncMap{"tile": tileFN}).
		Option("missingkey=error").
		Parse(string(in))
	if err != nil {
		return err
	}

	return t.Execute(dst, struct{}{})
}

// tileFunc is used both in pre-processing and is also available
// to Interpolator
func tileFunc(variables map[string]any) func() (string, error) {
	if variables == nil {
		variables = make(map[string]any)
	}

	return func() (string, error) {
		val, ok := variables[TileNameVariable]
		if !ok {
			return "", fmt.Errorf("could not find variable with key %q", TileNameVariable)
		}
		str, ok := val.(string)
		if !ok {
			return "", fmt.Errorf("variable %[1]q is %[2]T expected string: %[1]s=%[2]v", TileNameVariable, val)
		}
		return strings.ToLower(str), nil
	}
}
