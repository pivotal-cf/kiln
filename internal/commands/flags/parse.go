package flags

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type (
	HomeDirFunc func() (string, error)
	StatFunc    func(string) (os.FileInfo, error)

	FileSystem interface {
		billy.Basic
		billy.Dir
	}

	TileDirectory interface {
		TileDirectory() string
	}

	VariablesService interface {
		FromPathsAndPairs(paths []string, pairs []string) (templateVariables map[string]any, err error)
	}
)

type Standard struct {
	Kilnfile      string   `short:"kf"  long:"kilnfile"                   default:"Kilnfile"         description:"path to Kilnfile"`
	VariableFiles []string `short:"vf"  long:"variables-file"                                        description:"path to a file containing variables to interpolate"`
	Variables     []string `short:"vr"  long:"variable"                                              description:"key value pairs of variables to interpolate"`
}

type FetchBakeOptions struct {
	DownloadThreads              int  `short:"dt" long:"download-threads" description:"number of parallel threads to download parts from S3"`
	NoConfirm                    bool `short:"n" long:"no-confirm" default:"true" description:"non-interactive mode, will delete extra releases in releases dir without prompting"`
	AllowOnlyPublishableReleases bool `long:"allow-only-publishable-releases" default:"false" description:"include releases that would not be shipped with the tile (development builds)"`
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

func (options Standard) SaveKilnfileLock(fsOverride billy.Basic, kilnfileLock cargo.KilnfileLock) error {
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

func (options Standard) TileDirectory() string {
	if options.Kilnfile != "" {
		if filepath.Base(options.Kilnfile) == "Kilnfile" {
			return filepath.Dir(options.Kilnfile)
		}
		return options.Kilnfile
	}
	currentWorkingDirectory, _ := os.Getwd()
	return currentWorkingDirectory
}

// LoadWithDefaultFilePaths only sets default values if the flag is not set
// this permits explicitly setting "zero values" for in arguments without them being
// overwritten.
func LoadWithDefaultFilePaths(options TileDirectory, args []string, stat StatFunc) ([]string, error) {
	if stat == nil {
		stat = os.Stat
	}
	argsAfterFlags, err := jhanda.Parse(options, args)
	if err != nil {
		return nil, err
	}

	v := reflect.ValueOf(options).Elem()

	tileDir := options.TileDirectory()

	// handle simple case first
	configurePathDefaults(v, tileDir, args, stat)

	return argsAfterFlags, nil
}

func configurePathDefaults(v reflect.Value, pathPrefix string, args []string, stat StatFunc) {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		fieldType := t.Field(i)
		if !fieldType.IsExported() {
			continue
		}

		fieldValue := v.Field(i)

		switch fieldType.Type.Kind() {
		default:
			continue
		case reflect.Struct:
			configurePathDefaults(fieldValue, pathPrefix, args, stat)
			continue
		case reflect.String:
			defaultValue, ok := fieldType.Tag.Lookup("default")
			if !ok {
				continue
			}

			value := v.Field(i).Interface().(string)

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

			fieldValue.Set(reflect.ValueOf(value))
		case reflect.Slice:
			flagValues := v.Field(i).Interface().([]string)

			defaultValue, ok := fieldType.Tag.Lookup("default")
			if !ok {
				continue
			}
			defaultValues := strings.Split(defaultValue, ",")

			if len(flagValues) > len(defaultValues) && slices.Equal(flagValues[:len(defaultValues)], defaultValues) {
				fieldValue.Set(reflect.ValueOf(flagValues[len(defaultValues):]))
				continue
			}

			filteredDefaults := defaultValues[:0]
			for _, p := range defaultValues {
				p = strings.TrimSpace(p)
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
			fieldValue.Set(reflect.ValueOf(filteredDefaults))
		}
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

func Args(v any) []string {
	return encodeFlags(reflect.ValueOf(v))
}

func encodeFlags(v reflect.Value) []string {
	var result []string
	switch v.Kind() {
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			sv := v.Index(i)
			encode := encodeFlags(sv)
			result = append(result, encode...)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fieldVal := v.Field(i)
			fieldType := v.Type().Field(i)
			fieldAnnotation := fieldType.Tag.Get("long")
			if fieldAnnotation == "" {
				fieldAnnotation = fieldType.Tag.Get("short")
			}

			encode := encodeFlags(fieldVal)
			// TODO: make sure that valueless flags are passed correctly
			if fieldAnnotation != "" && fieldVal.Kind() == reflect.Bool {
				isSet := fieldVal.Bool()
				if isSet {
					result = append(result, "--"+fieldAnnotation)
				}
				continue
			}

			for _, enc := range encode {
				if fieldAnnotation != "" {
					result = append(result, "--"+fieldAnnotation, enc)
				} else {
					result = append(result, enc)
				}
			}
		}
	default:
		result = append(result, fmt.Sprintf("%v", v))
	}
	return result
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
