package cargo

import (
	"fmt"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
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

func LoadKilnfiles(kilnfilePath string, variablesFiles, variables []string) (Kilnfile, KilnfileLock, error) {
	templateVariablesService := baking.NewTemplateVariablesService()
	templateVariables, err := templateVariablesService.FromPathsAndPairs(variablesFiles, variables)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, fmt.Errorf("failed to parse template variables: %s", err)
	}

	kilnfileYAML, err := ioutil.ReadFile(kilnfilePath)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, err
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

	lockFileName := fmt.Sprintf("%s.lock", kilnfilePath)
	lockFile, err := os.Open(lockFileName)
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
