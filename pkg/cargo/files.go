package cargo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/builder"
)

func kilnfileLockPath(kilnfilePath string) string {
	return fmt.Sprintf("%s.lock", kilnfilePath)
}

type ConfigFileError struct {
	HumanReadableConfigFileName string
	err                         error
}

func (err ConfigFileError) Unwrap() error {
	return err.err
}

func (err ConfigFileError) Error() string {
	return fmt.Sprintf("encountered a configuration file error with %s: %s", err.HumanReadableConfigFileName, err.err.Error())
}

type KilnfileLoader struct{}

func (k KilnfileLoader) LoadKilnfiles(fs billy.Filesystem, kilnfilePath string, variablesFiles, variables []string) (Kilnfile, KilnfileLock, error) {
	templateVariablesService := baking.NewTemplateVariablesService(fs)
	templateVariables, err := templateVariablesService.FromPathsAndPairs(variablesFiles, variables)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, fmt.Errorf("error processing --variable or --variables-file arguments - are you logged into lpass? (error: %w)", err)
	}

	kf, err := fs.Open(kilnfilePath)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, fmt.Errorf("unable to open file %q: %w", kilnfilePath, err)
	}
	defer closeAndIgnoreError(kf)
	kilnfileYAML, err := io.ReadAll(kf)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, fmt.Errorf("unable to read file %q: %w", kilnfilePath, err)
	}

	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, "Kilnfile", kilnfileYAML)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, ConfigFileError{err: err, HumanReadableConfigFileName: "interpolating variable files with Kilnfile"}
	}

	var kilnfile Kilnfile
	err = yaml.Unmarshal(interpolatedMetadata, &kilnfile)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, ConfigFileError{err: err, HumanReadableConfigFileName: "Kilnfile specification " + kilnfilePath}
	}

	lockFileName := kilnfileLockPath(kilnfilePath)
	lockFile, err := fs.Open(lockFileName)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, err
	}
	defer closeAndIgnoreError(lockFile)

	var kilnfileLock KilnfileLock
	err = yaml.NewDecoder(lockFile).Decode(&kilnfileLock)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, ConfigFileError{err: err, HumanReadableConfigFileName: "Kilnfile.lock " + lockFileName}
	}
	return kilnfile, kilnfileLock, nil
}

func (KilnfileLoader) SaveKilnfileLock(fs billy.Filesystem, kilnfilePath string, updatedKilnfileLock KilnfileLock) error {
	updatedLockFileYAML, err := yaml.Marshal(updatedKilnfileLock)
	if err != nil {
		return fmt.Errorf("error marshaling the Kilnfile.lock: %w", err) // untestable
	}

	lockfilePath := kilnfileLockPath(kilnfilePath)
	lockFile, err := fs.Create(lockfilePath) // overwrites the file
	if err != nil {
		return fmt.Errorf("error reopening the Kilnfile.lock for writing: %w", err)
	}

	_, err = lockFile.Write(updatedLockFileYAML)
	if err != nil {
		return fmt.Errorf("error writing to Kilnfile.lock: %w", err)
	}

	return nil
}

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

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
