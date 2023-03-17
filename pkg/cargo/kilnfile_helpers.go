package cargo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	err = yaml.Unmarshal(interpolatedMetadata, &kilnfile)
	if err != nil {
		return Kilnfile{}, err
	}

	return kilnfile, nil
}

func FullKilnfilePath(path string) string {
	var kfPath string = path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		kfPath = filepath.Join(path, "Kilnfile")
	}

	return kfPath
}

func GetKilnfiles(path string) (Kilnfile, KilnfileLock, error) {
	kilnfile, err := GetKilnfile(path)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, err
	}
	kilnfileLock, err := GetKilnfileLock(path)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, err
	}

	return kilnfile, kilnfileLock, nil
}

func GetKilnfile(path string) (Kilnfile, error) {
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

func GetKilnfileLock(path string) (KilnfileLock, error) {
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

func WriteKilnfile(kf Kilnfile, path string) error {
	kfBytes, err := yaml.Marshal(kf)
	if err != nil {
		return err
	}

	outputKilnfile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(outputKilnfile)
	_, err = outputKilnfile.Write(kfBytes)
	return err
}
