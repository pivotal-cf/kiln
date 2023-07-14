package cargo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

func InterpolateAndParseKilnfile(in io.Reader, templateVariables map[string]any) (Kilnfile, error) {
	kilnfileYAML, err := io.ReadAll(in)
	if err != nil {
		return Kilnfile{}, fmt.Errorf("unable to read Kilnfile: %w", err)
	}

	kilnfileTemplate, err := template.New("Kilnfile").
		Funcs(template.FuncMap{
			"variable": variableTemplateFunction(templateVariables),
		}).
		Delims("$(", ")").
		Option("missingkey=error").
		Parse(string(kilnfileYAML))
	if err != nil {
		return Kilnfile{}, err
	}

	var buf bytes.Buffer
	if err := kilnfileTemplate.Execute(&buf, struct{}{}); err != nil {
		return Kilnfile{}, err
	}

	var kilnfile Kilnfile
	return kilnfile, yaml.Unmarshal(buf.Bytes(), &kilnfile)
}

func variableTemplateFunction(templateVariables map[string]any) func(name string) (string, error) {
	return func(name string) (string, error) {
		if templateVariables == nil {
			return "", errors.New("--variable or --variables-file must be specified")
		}
		val, ok := templateVariables[name]
		if !ok {
			return "", fmt.Errorf("could not find variable with key %q", name)
		}
		switch value := val.(type) {
		case string:
			return value, nil
		case int:
			return strconv.Itoa(value), nil
		default:
			return "", fmt.Errorf("the variables function used to interpolate variables in the Kilnfile only supports int and string values (%q has type %s)", name, value)
		}
	}
}

func ResolveKilnfilePath(path string) (string, error) {
	if ext := filepath.Ext(path); ext == ".lock" {
		path = strings.TrimSuffix(path, ".lock")
	}
	if filepath.Base(path) == "Kilnfile" {
		path = filepath.Dir(path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("expected a path to an existing directory that may/may-not contain a Kilnfile: %q is not a directory", path)
	}
	return filepath.Join(path, "Kilnfile"), nil
}

func ReadKilnfileAndKilnfileLock(path string) (Kilnfile, KilnfileLock, error) {
	kilnfile, err := ReadKilnfile(path)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, err
	}
	kilnfileLock, err := ReadKilnfileLock(path)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, err
	}

	return kilnfile, kilnfileLock, nil
}

func ReadKilnfile(path string) (Kilnfile, error) {
	kf, err := os.ReadFile(path)
	if err != nil {
		return Kilnfile{}, fmt.Errorf("failed to read Kilnfile: %w", err)
	}

	var kilnfile Kilnfile
	err = yaml.Unmarshal(kf, &kilnfile)
	if err != nil {
		return Kilnfile{}, fmt.Errorf("failed to unmarshall Kilnfile: %w", err)
	}

	return kilnfile, nil
}

func ReadKilnfileLock(path string) (KilnfileLock, error) {
	kfl, err := os.ReadFile(path + ".lock")
	if err != nil {
		return KilnfileLock{}, fmt.Errorf("failed to read Kilnfile.lock: %w", err)
	}

	var kilnfileLock KilnfileLock
	err = yaml.Unmarshal(kfl, &kilnfileLock)
	if err != nil {
		return KilnfileLock{}, fmt.Errorf("failed to unmarshall Kilnfile.lock: %w", err)
	}

	return kilnfileLock, nil
}

// WriteKilnfile does not validate the Kilnfile nor does it validate the path.
// Use ResolveKilnfilePath and maybe Validate before calling this.
func WriteKilnfile(path string, kf Kilnfile) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(f)
	e := yaml.NewEncoder(f)
	e.SetIndent(2)
	defer closeAndIgnoreError(e)
	return e.Encode(kf)
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}
