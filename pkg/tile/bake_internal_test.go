package tile

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBakeConfiguration_SetDefaults(t *testing.T) {
	t.Run("all_default_paths_exist", func(t *testing.T) {
		dir := fstest.MapFS{
			MetadataDefaultSourceFilePath: &fstest.MapFile{},
			VersionDefaultSourceFilePath: &fstest.MapFile{
				Data: []byte(`0.1.0`),
			},
		}
		configuration := new(BakeConfiguration)
		configuration.setDefaults(dir)
		assert.Equal(t, MetadataDefaultSourceFilePath, configuration.MetadataTemplateFilePath)
		assert.Equal(t, "0.1.0", configuration.TileVersion)
	})
	t.Run("version_is_missing", func(t *testing.T) {
		dir := fstest.MapFS{
			MetadataDefaultSourceFilePath: &fstest.MapFile{},
		}
		configuration := new(BakeConfiguration)
		configuration.setDefaults(dir)
		assert.Zero(t, configuration.TileVersion)
	})
}

func TestBakeConfiguration_Validate(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		err := (&BakeConfiguration{
			MetadataTemplateFilePath: "some-path",
			IconFilePath:             "some-path",
		}).validate()
		require.NoError(t, err)
	})
	t.Run("icon missing", func(t *testing.T) {
		err := (&BakeConfiguration{
			MetadataTemplateFilePath: "some-path",
		}).validate()
		require.Error(t, err)
	})
	t.Run("empty", func(t *testing.T) {
		err := (&BakeConfiguration{}).validate()
		require.Error(t, err)
	})
}
