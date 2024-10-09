package bake_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/bake"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestNew(t *testing.T) {
	outputDirectory := t.TempDir()

	err := bake.New(outputDirectory, filepath.FromSlash("testdata/tile-0.1.3.pivotal"), cargo.Kilnfile{})
	require.NoError(t, err)

	iconFileContents, _ := os.ReadFile(filepath.Join(outputDirectory, bake.DefaultFilepathIconImage))
	require.Equal(t, "some icon\n", string(iconFileContents), "it writes the icon image")

	baseYML, _ := os.ReadFile(filepath.Join(outputDirectory, bake.DefaultFilepathBaseYML))
	var productTemplate yaml.Node
	require.NoError(t, yaml.Unmarshal(baseYML, &productTemplate))

	// do kiln bake in outputDirectory and see if it outputs the same tile
}

func TestNew_bad_inputs(t *testing.T) {
	t.Run("not a zip file", func(t *testing.T) {
		outputDirectory := t.TempDir()
		const notAZipFile = "new_test.go"
		err := bake.New(outputDirectory, notAZipFile, cargo.Kilnfile{})
		require.ErrorContains(t, err, "zip")
	})

	t.Run("file does not exist", func(t *testing.T) {
		outputDirectory := t.TempDir()

		err := bake.New(outputDirectory, filepath.FromSlash("testdata/banana.pivotal"), cargo.Kilnfile{})
		require.ErrorContains(t, err, "no such file")
	})
}
