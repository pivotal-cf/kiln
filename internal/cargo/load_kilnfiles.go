package cargo

import (
	"fmt"
	"io/ioutil"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/yaml.v2"
)

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

type KilnfileLoader struct {
}

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
	defer kf.Close()
	kilnfileYAML, err := ioutil.ReadAll(kf)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, fmt.Errorf("unable to read file %q: %w", kilnfilePath, err)
	}

	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, kilnfileYAML)
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
	defer lockFile.Close()

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

func kilnfileLockPath(kilnfilePath string) string {
	return fmt.Sprintf("%s.lock", kilnfilePath)
}
