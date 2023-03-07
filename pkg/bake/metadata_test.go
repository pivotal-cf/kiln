package bake

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crhntr/yamlutil/yamlnode"
	"github.com/crhntr/yamlutil/yamltest"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMetadata(t *testing.T) {
	if _, err := os.Stat(filepath.Join("testdata", "hello-tile", "base.yml")); errors.Is(err, os.ErrNotExist) {
		t.Skipf("skipping metadata tests becuase the hello-tile submodule has not been fetched")
	}

	t.Cleanup(func() {
		resetHelloTile(t)
	})
	t.Setenv("BLOCK_PARALLEL_TEST_EXEC", "this makes the test fail if the tests are not called in sequence")

	t.Run("hello_tile", func(t *testing.T) {
		helloTileDirectory := resetHelloTile(t)
		_, err := Metadata(os.DirFS(helloTileDirectory), Options{})
		require.NoError(t, err)
	})

	t.Run("non standard jobs directory", func(t *testing.T) {
		helloTileDirectory := resetHelloTile(t)

		require.NoError(t, os.Rename(filepath.Join(helloTileDirectory, "jobs"), filepath.Join(helloTileDirectory, "bananas")))

		_, err := Metadata(os.DirFS(helloTileDirectory), Options{
			Jobs: []string{"bananas"},
		})
		require.NoError(t, err)
	})

	t.Run("non standard instance_groups directory", func(t *testing.T) {
		helloTileDirectory := resetHelloTile(t)

		require.NoError(t, os.Rename(filepath.Join(helloTileDirectory, "instance_groups"), filepath.Join(helloTileDirectory, "bananas")))

		_, err := Metadata(os.DirFS(helloTileDirectory), Options{
			InstanceGroups: []string{"bananas"},
		})
		require.NoError(t, err)
	})

	t.Run("non standard metadata file", func(t *testing.T) {
		helloTileDirectory := resetHelloTile(t)

		require.NoError(t, os.Rename(filepath.Join(helloTileDirectory, "base.yml"), filepath.Join(helloTileDirectory, "metadata.yml")))

		_, err := Metadata(os.DirFS(helloTileDirectory), Options{
			Metadata: "metadata.yml",
		})
		require.NoError(t, err)
	})

	t.Run("non icon file metadata file", func(t *testing.T) {
		helloTileDirectory := resetHelloTile(t)

		require.NoError(t, os.Rename(filepath.Join(helloTileDirectory, "icon.png"), filepath.Join(helloTileDirectory, "logo.png")))

		_, err := Metadata(os.DirFS(helloTileDirectory), Options{
			Icon: "logo.png",
		})
		require.NoError(t, err)
	})

	t.Run("non standard version file", func(t *testing.T) {
		helloTileDirectory := resetHelloTile(t)

		require.NoError(t, os.Rename(filepath.Join(helloTileDirectory, "version"), filepath.Join(helloTileDirectory, "banana")))

		_, err := Metadata(os.DirFS(helloTileDirectory), Options{
			VersionFile: "banana",
		})
		require.NoError(t, err)
	})

	t.Run("version value takes precedent over a version file", func(t *testing.T) {
		helloTileDirectory := resetHelloTile(t)

		const versionValue = "0.99.0"
		metadataBytes, err := Metadata(os.DirFS(helloTileDirectory), Options{
			VersionValue: versionValue,
		})
		require.NoError(t, err)

		var m struct {
			ProductVersion string `yaml:"product_version"`
		}
		require.NoError(t, yaml.Unmarshal(metadataBytes, &m))

		require.Equal(t, versionValue, m.ProductVersion)
	})

	t.Run("no variables files are provided", func(t *testing.T) {
		helloTileDirectory := resetHelloTile(t)

		require.NoError(t, os.RemoveAll(filepath.Join(helloTileDirectory, "variables")))

		_, err := Metadata(os.DirFS(helloTileDirectory), Options{})
		require.NoError(t, err)
	})
}

func TestMetadata_CompareMetadataWithLegacyKilnBake(t *testing.T) {
	tasRepoPath, found := os.LookupEnv("TAS_DIRECTORY")
	if !found {
		t.Skip("missing environment variable TAS_DIRECTORY")
	}

	for _, tt := range []struct {
		Name, Directory string
		VariablesFile   string
	}{
		{Name: "srt", Directory: "tas", VariablesFile: filepath.Join("variables", "srt.yml")},
		{Name: "ert", Directory: "tas", VariablesFile: filepath.Join("variables", "ert.yml")},
		{Name: "ist", Directory: "ist", VariablesFile: filepath.Join("variables", "ist.yml")},
		{Name: "wrt", Directory: "tasw", VariablesFile: filepath.Join("variables", "wrt.yml")},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			tileDirectory := filepath.Join(tasRepoPath, tt.Directory)
			tasTileDirFS := os.DirFS(tileDirectory)

			var (
				oldMetadataBuffer        bytes.Buffer
				newMetadata, oldMetadata yaml.Node
				o                        = Options{
					TileName: tt.Name,
				}
			)
			require.NoError(t, EnsureVariableGitMetaDataSHA(&o, tileDirectory))
			newMetadataBytes, err := Metadata(tasTileDirFS, o)
			require.NoError(t, err)

			require.NoError(t, yaml.Unmarshal(newMetadataBytes, &newMetadata))

			var kilnStdErr bytes.Buffer
			kilnBake := exec.Command("kiln", "bake", "--metadata-only", "--stub-releases", "--variables-file", tt.VariablesFile, "--variable=git_metadata_sha=development")
			kilnBake.Stdout = &oldMetadataBuffer
			kilnBake.Stderr = &kilnStdErr
			kilnBake.Dir = tileDirectory
			require.NoError(t, kilnBake.Run(), kilnStdErr.String())
			require.NoError(t, yaml.Unmarshal(oldMetadataBuffer.Bytes(), &oldMetadata))

			trimSpaceOnMultiLineValues(t, &oldMetadata)
			trimSpaceOnMultiLineValues(t, &newMetadata)

			yamltest.AssertEqual(t, &oldMetadata, &newMetadata)
		})
	}
}

func trimSpaceOnMultiLineValues(t *testing.T, node *yaml.Node) {
	t.Helper()
	require.NoError(t, yamlnode.Walk(node, func(node *yaml.Node) error {
		if node.Tag != "!!str" {
			return nil
		}
		if strings.Count(node.Value, "\n") > 1 {
			node.Value = strings.TrimSpace(node.Value)
		}
		return nil
	}, yaml.ScalarNode))
}

func resetHelloTile(t *testing.T) string {
	helloTileDirectory := filepath.Join("testdata", "hello-tile")
	for _, gitArgs := range [][]string{
		{"restore", "--recurse-submodules", "."},
		{"clean", "-ffd"},
	} {
		git := exec.Command("git", gitArgs...)
		git.Dir = helloTileDirectory
		require.NoErrorf(t, git.Run(), "failed git command with args: %s", strings.Join(gitArgs, " "))
	}
	return helloTileDirectory
}
