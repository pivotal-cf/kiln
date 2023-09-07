package flags_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

func TestLoadFlagsWithDefaults(t *testing.T) {
	t.Run("when the default directory exists", func(t *testing.T) {
		var options struct {
			testTileDir
			SomeDirectory string `long:"some-directory" default:"basket"`
		}
		options.testTileDir.filePath = t.TempDir()

		someDirPath := filepath.Join(options.testTileDir.filePath, "basket")
		err := os.MkdirAll(someDirPath, 0777)
		require.NoError(t, err)

		var statParam string
		_, err = flags.LoadWithDefaults(&options, nil, func(s string) (os.FileInfo, error) {
			statParam = s
			return os.Stat(s)
		})
		require.NoError(t, err)
		assert.Equal(t, someDirPath, statParam, "it checks that the directory exists")
		assert.Equal(t, someDirPath, options.SomeDirectory, "it sets the struct field")
	})

	t.Run("when the default file exists", func(t *testing.T) {
		var options struct {
			testTileDir
			SomeConfig string `long:"some-config" default:"config.txt"`
		}
		options.testTileDir.filePath = t.TempDir()

		someConfigPath := filepath.Join(options.testTileDir.filePath, "config.txt")
		f, err := os.Create(someConfigPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		var statParam string
		_, err = flags.LoadWithDefaults(&options, nil, func(s string) (os.FileInfo, error) {
			statParam = s
			return os.Stat(s)
		})
		require.NoError(t, err)
		assert.Equal(t, someConfigPath, statParam, "it checks that the directory exists")
		assert.Equal(t, someConfigPath, options.SomeConfig, "it sets the struct field")
	})

	t.Run("when the default directory does not exist", func(t *testing.T) {
		var options struct {
			testTileDir
			SomeDirectory string `long:"some-directory" default:"basket"`
		}
		options.testTileDir.filePath = t.TempDir()

		someDirPath := filepath.Join(options.testTileDir.filePath, "basket")

		var statParam string
		_, err := flags.LoadWithDefaults(&options, nil, func(s string) (os.FileInfo, error) {
			statParam = s
			return os.Stat(s)
		})
		require.NoError(t, err)
		assert.Equal(t, someDirPath, statParam, "it checks that the directory exists")
		assert.Zero(t, options.SomeDirectory, "it does not set the field")
	})

	t.Run("when the default file does not exist", func(t *testing.T) {
		var options struct {
			testTileDir
			SomeConfig string `long:"some-config" default:"config.txt"`
		}
		options.testTileDir.filePath = t.TempDir()

		someConfigPath := filepath.Join(options.testTileDir.filePath, "config.txt")

		var statParam string
		_, err := flags.LoadWithDefaults(&options, nil, func(s string) (os.FileInfo, error) {
			statParam = s
			return os.Stat(s)
		})
		require.NoError(t, err)
		assert.Equal(t, someConfigPath, statParam, "it checks that the directory exists")
		assert.Zero(t, options.SomeConfig, "it does not set the field")
	})

	t.Run("when the flag is set", func(t *testing.T) {
		var options struct {
			testTileDir
			SomeConfig string `long:"some-config" default:"config.txt"`
		}
		options.testTileDir.filePath = t.TempDir()

		otherDir := t.TempDir()
		optionsFilePath := filepath.Join(otherDir, "options.xml")

		var statCallCount int
		_, err := flags.LoadWithDefaults(&options, []string{"--some-config", optionsFilePath}, func(s string) (os.FileInfo, error) {
			statCallCount++
			return os.Stat(s)
		})
		require.NoError(t, err)
		assert.Zero(t, statCallCount, "it does not check if the directory exists")
		assert.Equal(t, optionsFilePath, options.SomeConfig)
	})

	t.Run("when a non-string field exists", func(t *testing.T) {
		var options struct {
			testTileDir
			Count int `long:"some-count" default:"8"`
		}
		options.testTileDir.filePath = t.TempDir()
		_, err := flags.LoadWithDefaults(&options, nil, nil)
		assert.NoError(t, err)
	})

	t.Run("when a field is not tagged with a default value", func(t *testing.T) {
		var options struct {
			testTileDir
			SomeConfig string `long:"some-config"`
		}
		options.testTileDir.filePath = t.TempDir()

		var statCallCount int

		_, err := flags.LoadWithDefaults(&options, nil, func(s string) (os.FileInfo, error) {
			statCallCount++
			return os.Stat(s)
		})
		assert.NoError(t, err)
		assert.Zero(t, statCallCount, "it does not call stat")
	})

	t.Run("when a field has a fancy string type", func(t *testing.T) {
		type Rope string
		var options struct {
			testTileDir
			Lifeline Rope `long:"some-help" default:"polypropylene"`
		}
		options.testTileDir.filePath = t.TempDir()
		assert.Panics(t, func() {
			_, _ = flags.LoadWithDefaults(&options, nil, nil)
		}, "jhana panics")
	})
}

type testTileDir struct {
	filePath string
}

func (ttd testTileDir) TileDirectory() string {
	return ttd.filePath
}

func TestArgs(t *testing.T) {
	t.Run("when booleans are true", func(t *testing.T) {
		options := struct {
			flags.Standard
			flags.FetchBakeOptions
			commands.FetchReleaseDir
		}{
			flags.Standard{Kilnfile: "kilnfile1", VariableFiles: []string{"variables-files-1", "variables-files-2"}, Variables: []string{"variables-1", "variables-2"}},
			flags.FetchBakeOptions{
				DownloadThreads:              0,
				NoConfirm:                    true,
				AllowOnlyPublishableReleases: false,
			},
			commands.FetchReleaseDir{ReleasesDir: "releases-dir"},
		}

		got := flags.Args(options)

		assert.Equal(t, got, []string{"--kilnfile", "kilnfile1",
			"--variables-file", "variables-files-1",
			"--variables-file", "variables-files-2",
			"--variable", "variables-1",
			"--variable", "variables-2",
			"--download-threads", "0",
			"--no-confirm",
			"--releases-directory", "releases-dir",
		}, "it encodes an options struct into a string slice with jhanda formatting")
	})

	t.Run("when booleans are false", func(t *testing.T) {
		options := struct {
			flags.Standard
			flags.FetchBakeOptions
			commands.FetchReleaseDir
		}{
			flags.Standard{Kilnfile: "kilnfile1", VariableFiles: []string{"variables-files-1", "variables-files-2"}, Variables: []string{"variables-1", "variables-2"}},
			flags.FetchBakeOptions{
				DownloadThreads:              0,
				NoConfirm:                    false,
				AllowOnlyPublishableReleases: false,
			},
			commands.FetchReleaseDir{ReleasesDir: "releases-dir"},
		}

		args := flags.Args(options)

		assert.Equal(t, args, []string{
			"--kilnfile", "kilnfile1",
			"--variables-file", "variables-files-1",
			"--variables-file", "variables-files-2",
			"--variable", "variables-1",
			"--variable", "variables-2",
			"--download-threads", "0",
			"--releases-directory", "releases-dir",
		}, "it encodes an options struct into a string slice with jhanda formatting")
	})
}
