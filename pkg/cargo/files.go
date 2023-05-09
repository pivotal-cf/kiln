package cargo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/builder"
)

func InterpolateAndParseKilnfile(in io.Reader, templateVariables map[string]interface{}) (Kilnfile, error) {
	kilnfileYAML, err := io.ReadAll(in)
	if err != nil {
		return Kilnfile{}, fmt.Errorf("unable to read Kilnfile: %w", err)
	}

	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, "Kilnfile", kilnfileYAML)
	if err != nil {
		return Kilnfile{}, err
	}

	var kilnfile Kilnfile
	return kilnfile, yaml.Unmarshal(interpolatedMetadata, &kilnfile)
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
		return "", fmt.Errorf("kilnfile invalid expected a path to a Kilnfile")
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

func WriteKilnfile(path string, kf Kilnfile) error {
	if filepath.Base(path) != "Kilnfile" {
		path = filepath.Join(path, "Kilnfile")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(f)
	e := yaml.NewEncoder(f)
	defer closeAndIgnoreError(e)
	return e.Encode(kf)
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}
