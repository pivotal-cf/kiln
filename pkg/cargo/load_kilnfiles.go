package cargo

import (
	"fmt"
	"io"

	"github.com/go-git/go-billy/v5"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/builder"
)

type KilnfileLoader struct{}

func (k KilnfileLoader) LoadKilnfiles(fs billy.Filesystem, kilnfilePath string, variablesFiles, variables []string) (Kilnfile, KilnfileLock, error) {
	templateVariablesService := baking.NewTemplateVariablesService(fs)
	templateVariables, err := templateVariablesService.FromPathsAndPairs(variablesFiles, variables)
	if err != nil {
		return Kilnfile{}, KilnfileLock{}, fmt.Errorf("error processing variables: %w", err)
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

	if int64(kilnfile.MajorVersion) != Version().Major() {
		return Kilnfile{}, KilnfileLock{}, fmt.Errorf("kiln_major_version %d in Kilnfile does not match the kiln major version %d", kilnfile.MajorVersion, Version().Major())
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

func kilnfileLockPath(kilnfilePath string) string {
	return fmt.Sprintf("%s.lock", kilnfilePath)
}
