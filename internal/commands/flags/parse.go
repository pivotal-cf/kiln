package flags

import (
	"fmt"
	"go/ast"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

type (
	StatFunc func(string) (os.FileInfo, error)

	KilnfileOptions interface {
		KilnfilePathPrefix() string
	}
)

//counterfeiter:generate -o ../fakes/variables_service.go --fake-name VariablesService . VariablesService
type VariablesService interface {
	FromPathsAndPairs(paths []string, pairs []string) (templateVariables map[string]interface{}, err error)
}

type Standard struct {
	Kilnfile      string   `short:"kf"  long:"kilnfile"                   default:"Kilnfile"         description:"path to Kilnfile"`
	VariableFiles []string `short:"vf"  long:"variables-file"                                        description:"path to a file containing variables to interpolate"`
	Variables     []string `short:"vr"  long:"variable"                                              description:"key value pairs of variables to interpolate"`
}

func defaults(fs billy.Basic, vs VariablesService, run RunFunc) (billy.Basic, VariablesService, RunFunc) {
	if fs == nil {
		fs = osfs.New("")
	}
	if run == nil {
		run = func(stdOut io.Writer, cmd *exec.Cmd) error {
			cmd.Stdout = stdOut
			return cmd.Run()
		}
	}
	if vs == nil {
		vs = baking.NewTemplateVariablesService(fs)
	}
	return fs, vs, run
}

type RunFunc func(stdOut io.Writer, cmd *exec.Cmd) error

// LoadKilnfiles parses and interpolates the Kilnfile and parsed the Kilnfile.lock.
// The function parameters are for overriding default services. These parameters are
// helpful for testing, in most cases nil can be passed for both.
func (std *Standard) LoadKilnfiles(fs billy.Basic, vs VariablesService, run RunFunc) (cargo.Kilnfile, cargo.KilnfileLock, error) {
	fs, vs, run = defaults(fs, vs, run)

	templateVariables, err := vs.FromPathsAndPairs(std.VariableFiles, std.Variables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, fmt.Errorf("failed to parse template variables: %s", err)
	}

	kilnfileFP, err := fs.Open(std.Kilnfile)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}
	defer func() {
		_ = kilnfileFP.Close()
	}()

	kilnfile, err := cargo.InterpolateAndParseKilnfile(kilnfileFP, templateVariables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	lockFP, err := fs.Open(std.KilnfileLockPath())
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
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

func (std *Standard) SaveKilnfileLock(fs billy.Basic, kilnfileLock cargo.KilnfileLock) error {
	fs, _, _ = defaults(fs, nil, nil)

	updatedLockFileYAML, err := yaml.Marshal(kilnfileLock)
	if err != nil {
		return fmt.Errorf("error marshaling the Kilnfile.lock: %w", err) // untestable
	}

	lockFile, err := fs.Create(std.KilnfileLockPath()) // overwrites the file
	if err != nil {
		return fmt.Errorf("error reopening the Kilnfile.lock for writing: %w", err)
	}

	_, err = lockFile.Write(updatedLockFileYAML)
	if err != nil {
		return fmt.Errorf("error writing to Kilnfile.lock: %w", err)
	}

	return nil
}

func (std Standard) KilnfilePathPrefix() string {
	pathPrefix := filepath.Dir(std.Kilnfile)
	if pathPrefix == "." {
		pathPrefix = ""
	}
	return pathPrefix
}

func (std Standard) KilnfileLockPath() string {
	return std.Kilnfile + ".lock"
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

func mergeMaps(dst, src map[string]interface{}) {
	for k, v := range src {
		dst[k] = v
	}
}
