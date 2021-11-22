package options

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pivotal-cf/jhanda"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

type (
	StatFunc func(string) (os.FileInfo, error)

	KilnfilePathPrefixer interface {
		KilnfilePathPrefix() string
	}
)

type Standard struct {
	Kilnfile      string   `short:"kf"  long:"kilnfile"                   default:"Kilnfile"         description:"path to Kilnfile"`
	VariableFiles []string `short:"vf"  long:"variables-file"                                        description:"path to a file containing variables to interpolate"`
	Variables     []string `short:"vr"  long:"variable"                                              description:"key value pairs of variables to interpolate"`
}

func (std Standard) EmbeddedStandardOptions() Standard { return std }

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

type StandardOptionsEmbedder interface {
	EmbeddedStandardOptions() Standard
}

// FlagsWithDefaults only sets default values if the flag is not set
// this permits explicitly setting "zero values" for in arguments without them being
// overwritten.
func FlagsWithDefaults(options StandardOptionsEmbedder, args []string, statOverride StatFunc) ([]string, error) {
	if statOverride == nil {
		statOverride = os.Stat
	}
	argsAfterFlags, err := jhanda.Parse(options, args)
	if err != nil {
		return nil, err
	}

	v := reflect.ValueOf(options).Elem()

	pathPrefix := options.EmbeddedStandardOptions().KilnfilePathPrefix()

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
			if field.Anonymous && token.IsExported(embeddedValue.Type().Name()) {
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
