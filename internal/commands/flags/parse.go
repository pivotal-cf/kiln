package flags

import (
	"fmt"
	"go/ast"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type (
	StatFunc func(string) (os.FileInfo, error)

	KilnfileOptions interface {
		KilnfilePathPrefix() string
	}

	VariablesService interface {
		FromPathsAndPairs(paths []string, pairs []string) (templateVariables map[string]interface{}, err error)
	}
)

type Standard struct {
	Kilnfile      string   `short:"kf"  long:"kilnfile"                   default:"Kilnfile"         description:"path to Kilnfile"`
	VariableFiles []string `short:"vf"  long:"variables-file"                                        description:"path to a file containing variables to interpolate"`
	Variables     []string `short:"vr"  long:"variable"                                              description:"key value pairs of variables to interpolate"`
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
	defer func() {
		_ = kilnfileFP.Close()
	}()

	kilnfile, err := cargo.InterpolateAndParseKilnfile(kilnfileFP, templateVariables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	err = kilnfile.ReleaseSources.Validate()
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	lockFP, err := fs.Open(options.KilnfileLockPath())
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, fmt.Errorf("failed to open Kilnfile.lock: %w", err)
	}
	defer func() {
		_ = lockFP.Close()
	}()
	lockBuf, err := ioutil.ReadAll(lockFP)
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

func (options Standard) KilnfilePathPrefix() string {
	pathPrefix := filepath.Dir(options.Kilnfile)
	if pathPrefix == "." {
		pathPrefix = ""
	}
	return pathPrefix
}

func (options Standard) KilnfileLockPath() string {
	return options.Kilnfile + ".lock"
}

// LoadFlagsWithDefaults only sets default values if the flag is not set
// this permits explicitly setting "zero values" for in arguments without them being
// overwritten.
func LoadFlagsWithDefaults(options KilnfileOptions, args []string, statOverride StatFunc) ([]string, error) {
	if statOverride == nil {
		statOverride = os.Stat
	}
	argsAfterFlags, err := jhanda.Parse(options, args)
	if err != nil {
		return nil, err
	}

	v := reflect.ValueOf(options).Elem()

	pathPrefix := options.KilnfilePathPrefix()

	// handle simple case first
	configureArrayDefaults(v, pathPrefix, args, statOverride)
	configurePathDefaults(v, pathPrefix, args, statOverride)

	return argsAfterFlags, nil
}

func configureArrayDefaults(v reflect.Value, pathPrefix string, args []string, stat StatFunc) {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		switch field.Type.Kind() {
		default:
			continue
		case reflect.Struct:
			embeddedValue := v.Field(i)
			if field.Anonymous && ast.IsExported(embeddedValue.Type().Name()) {
				configurePathDefaults(embeddedValue, pathPrefix, args, stat)
			}
			continue
		case reflect.Slice:
		}

		defaultValueStr, ok := field.Tag.Lookup("default")
		if !ok {
			continue
		}
		defaultValues := strings.Split(defaultValueStr, ",")

		flagValues, ok := v.Field(i).Interface().([]string)
		if !ok {
			// this might occur if we add non string slice params
			// notice the field Kind check above was not super specific
			continue
		}

		if IsSet(field.Tag.Get("short"), field.Tag.Get("long"), args) {
			v.Field(i).Set(reflect.ValueOf(flagValues[len(defaultValues):]))
			continue
		}

		filteredDefaults := defaultValues[:0]
		for _, p := range defaultValues {
			if pathPrefix != "" {
				p = filepath.Join(pathPrefix, p)
			}
			_, err := stat(p)
			if err != nil {
				continue
			}
			filteredDefaults = append(filteredDefaults, p)
		}

		// if default values were found, use them,
		// else filteredDefaults will be an empty slice
		//   and the Bake command will continue as if they were not set
		v.Field(i).Set(reflect.ValueOf(filteredDefaults))
	}
}

func configurePathDefaults(v reflect.Value, pathPrefix string, args []string, stat StatFunc) {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		switch field.Type.Kind() {
		default:
			continue
		case reflect.Struct:
			embeddedValue := v.Field(i)
			if field.Anonymous && ast.IsExported(embeddedValue.Type().Name()) {
				configurePathDefaults(embeddedValue, pathPrefix, args, stat)
			}
			continue
		case reflect.String:
		}

		if IsSet(field.Tag.Get("short"), field.Tag.Get("long"), args) {
			continue
		}

		defaultValue, ok := field.Tag.Lookup("default")
		if !ok {
			continue
		}

		value, ok := v.Field(i).Interface().(string)
		if !ok {
			continue // this should not occur
		}

		isDefaultValue := defaultValue == value

		if !isDefaultValue {
			continue
		}

		if pathPrefix != "" {
			value = filepath.Join(pathPrefix, value)
		}

		_, err := stat(value)
		if err != nil {
			// set to zero value
			v.Field(i).Set(reflect.Zero(v.Field(i).Type()))
			continue
		}

		v.Field(i).Set(reflect.ValueOf(value))
	}
}

// IsSet can be used to check if a flag is set in a set
// of arguments. Both "long" and "short" flag names must
// be passed.
func IsSet(short, long string, args []string) bool {
	check := func(name string, arg string) bool {
		if name == "" {
			return false
		}

		return arg == "--"+name || arg == "-"+name ||
			strings.HasPrefix(arg, "--"+name+"=") ||
			strings.HasPrefix(arg, "-"+name+"=")
	}

	for _, a := range args {
		if check(short, a) || check(long, a) {
			return true
		}
	}

	return false
}
