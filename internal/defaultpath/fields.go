package defaultpath

import (
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const TagName = "default-path"

type StatFunc func(string) (os.FileInfo, error)

func SetFields(config interface{}, pathPrefix string, args []string, stat StatFunc) {
	if stat == nil {
		stat = os.Stat
	}
	v := reflect.ValueOf(config).Elem()

	configureArrayDefaults(v, pathPrefix, args, stat)
	configurePathDefaults(v, pathPrefix, args, stat)
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
				configureArrayDefaults(embeddedValue, pathPrefix, args, stat)
			}
			continue
		case reflect.Slice:
		}

		defaultValueStr, ok := field.Tag.Lookup(TagName)
		if !ok {
			continue
		}
		defaultValues := filter(strings.Split(defaultValueStr, ","), "", -1)

		passedValues, ok := v.Field(i).Interface().([]string)
		if !ok {
			// this might occur if we add non string slice params
			// notice the field Kind check above was not super specific
			continue
		}
		passedValues = filter(passedValues, "", -1)

		if IsSet(field.Tag.Get("long"), field.Tag.Get("short"), args) {
			v.Field(i).Set(reflect.ValueOf(passedValues))
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

		defaultValue, ok := field.Tag.Lookup(TagName)
		if !ok {
			continue
		}

		value, ok := v.Field(i).Interface().(string)
		if !ok {
			continue // this should not occur
		}

		isDefaultValue := defaultValue == value

		if isDefaultValue {
			continue
		}
		value = defaultValue

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

func filter[T comparable](in []T, valueToRemove T, limit int) []T {
	filtered := in[:0]
	removed := 0
	for _, v := range in {
		if v == valueToRemove {
			continue
		}
		filtered = append(filtered, v)
		removed++
		if limit > 0 && removed < limit {
			break
		}
	}
	return filtered
}
