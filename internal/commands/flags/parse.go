package flags

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/defaultpath"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type (
	StatFunc = defaultpath.StatFunc

	KilnfileOptions interface {
		KilnfilePathPrefix() string
	}

	VariablesService interface {
		FromPathsAndPairs(paths []string, pairs []string) (templateVariables map[string]interface{}, err error)
	}
)

type Standard struct {
	Kilnfile      string   `short:"kf"  long:"kilnfile"       default-path:"Kilnfile" description:"path to Kilnfile"`
	VariableFiles []string `short:"vf"  long:"variables-file"                         description:"path to a file containing variables to interpolate"`
	Variables     []string `short:"vr"  long:"variable"                               description:"key value pairs of variables to interpolate"`
}

// LoadKilnfiles parses and interpolates the Kilnfile and parsed the Kilnfile.lock.
// The function parameters are for overriding default services. These parameters are
// helpful for testing, in most cases nil can be passed for both.
func (options *Standard) LoadKilnfiles(fsOverride billy.Basic, variablesServiceOverride VariablesService) (_ cargo.Kilnfile, _ cargo.KilnfileLock, err error) {
	fs := fsOverride
	if fs == nil {
		fs = osfs.New("")
	}
	variablesService := variablesServiceOverride
	if variablesService == nil {
		variablesService = baking.NewTemplateVariablesService(fs)
	}

	templateVariables, err := variablesService.FromPathsAndPairs(options.VariableFiles, options.Variables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, fmt.Errorf("failed to parse template variables: %s", err)
	}

	kilnfileFP, err := fs.Open(options.Kilnfile)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, fmt.Errorf("failed to open Kilnfile: %w", err)
	}
	defer closeAndIgnoreError(kilnfileFP)

	kilnfile, err := cargo.InterpolateAndParseKilnfile(kilnfileFP, templateVariables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	lockFP, err := fs.Open(options.KilnfileLockPath())
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, fmt.Errorf("failed to open Kilnfile.lock: %w", err)
	}
	defer closeAndIgnoreError(lockFP)
	lockBuf, err := io.ReadAll(lockFP)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	var lock cargo.KilnfileLock
	err = yaml.Unmarshal(lockBuf, &lock)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	return kilnfile, lock, nil
}

func (options *Standard) SaveKilnfileLock(fsOverride billy.Basic, kilnfileLock cargo.KilnfileLock) error {
	fs := fsOverride
	if fs == nil {
		fs = osfs.New("")
	}

	updatedLockFileYAML, err := yaml.Marshal(kilnfileLock)
	if err != nil {
		return fmt.Errorf("error marshaling the Kilnfile.lock: %w", err) // untestable
	}

	lockFile, err := fs.Create(options.KilnfileLockPath()) // overwrites the file
	if err != nil {
		return fmt.Errorf("error reopening the Kilnfile.lock for writing: %w", err)
	}

	_, err = lockFile.Write(updatedLockFileYAML)
	if err != nil {
		return fmt.Errorf("error writing to Kilnfile.lock: %w", err)
	}

	return nil
}

func (options *Standard) KilnfilePathPrefix() string {
	pathPrefix := filepath.Dir(options.Kilnfile)
	if pathPrefix == "." {
		pathPrefix = ""
	}
	return pathPrefix
}

func (options *Standard) KilnfileLockPath() string {
	return options.Kilnfile + ".lock"
}

// LoadFlagsWithDefaults only sets default values if the flag is not set
// this permits explicitly setting "zero values" for in arguments without them being
// overwritten.
func LoadFlagsWithDefaults(options KilnfileOptions, args []string, statOverride defaultpath.StatFunc) ([]string, error) {
	argsAfterFlags, err := jhanda.Parse(options, args)
	if err != nil {
		return nil, err
	}

	defaultpath.SetFields(options, options.KilnfilePathPrefix(), args, statOverride)

	return argsAfterFlags, nil
}

// IsSet can be used to check if a flag is set in a set
// of arguments. Both "long" and "short" flag names must
// be passed.
func IsSet(short, long string, args []string) bool {
	return defaultpath.IsSet(short, long, args)
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
