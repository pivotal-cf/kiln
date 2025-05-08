package bake_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/pkg/bake"
)

func TestBakeOptions(t *testing.T) {
	options := reflect.TypeOf(commands.BakeOptions{})

	for _, tt := range []struct {
		Constant string
		LongFlag string
	}{
		{
			Constant: bake.DefaultFilepathIconImage,
			LongFlag: "icon",
		},
		{
			Constant: bake.DefaultFilepathKilnfile,
			LongFlag: "Kilnfile",
		},
		{
			Constant: bake.DefaultFilepathBaseYML,
			LongFlag: "metadata",
		},
		{
			Constant: bake.DefaultDirectoryReleases,
			LongFlag: "releases-directory",
		},
		{
			Constant: bake.DefaultDirectoryForms,
			LongFlag: "forms-directory",
		},
		{
			Constant: bake.DefaultDirectoryInstanceGroups,
			LongFlag: "instance-groups-directory",
		},
		{
			Constant: bake.DefaultDirectoryJobs,
			LongFlag: "jobs-directory",
		},
		{
			Constant: bake.DefaultDirectoryMigrations,
			LongFlag: "migrations-directory",
		},
		{
			Constant: bake.DefaultDirectoryProperties,
			LongFlag: "properties-directory",
		},
		{
			Constant: bake.DefaultDirectoryRuntimeConfigs,
			LongFlag: "runtime-configs-directory",
		},
		{
			Constant: bake.DefaultDirectoryBOSHVariables,
			LongFlag: "bosh-variables-directory",
		},
	} {
		t.Run(tt.Constant, func(t *testing.T) {
			field, found := fieldByTag(options, "long", "icon")
			require.True(t, found)
			require.Equal(t, field.Tag.Get("default"), bake.DefaultFilepathIconImage)
		})
	}
}

func fieldByTag(tp reflect.Type, tagName, tagValue string) (reflect.StructField, bool) {
	for i := 0; i < tp.NumField(); i++ {
		field := tp.Field(i)
		value, ok := field.Tag.Lookup(tagName)
		if ok && value == tagValue {
			return field, true
		}
	}
	return reflect.StructField{}, false
}
