package flags_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

func TestLoadFlagsWithDefaults(t *testing.T) {
	t.Run("when the default directory exists", func(t *testing.T) {
		var options struct {
			testTileDir
			SomeDirectory string `long:"some-directory" default:"basket"`
		}
		options.filePath = t.TempDir()

		someDirPath := filepath.Join(options.filePath, "basket")
		err := os.MkdirAll(someDirPath, 0o777)
		require.NoError(t, err)

		var statParam string
		_, err = flags.LoadWithDefaultFilePaths(&options, nil, func(s string) (os.FileInfo, error) {
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
		options.filePath = t.TempDir()

		someConfigPath := filepath.Join(options.filePath, "config.txt")
		f, err := os.Create(someConfigPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		var statParam string
		_, err = flags.LoadWithDefaultFilePaths(&options, nil, func(s string) (os.FileInfo, error) {
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
		options.filePath = t.TempDir()

		someDirPath := filepath.Join(options.filePath, "basket")

		var statParam string
		_, err := flags.LoadWithDefaultFilePaths(&options, nil, func(s string) (os.FileInfo, error) {
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
		options.filePath = t.TempDir()

		someConfigPath := filepath.Join(options.filePath, "config.txt")

		var statParam string
		_, err := flags.LoadWithDefaultFilePaths(&options, nil, func(s string) (os.FileInfo, error) {
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
		options.filePath = t.TempDir()

		otherDir := t.TempDir()
		optionsFilePath := filepath.Join(otherDir, "options.xml")

		var statCallCount int
		_, err := flags.LoadWithDefaultFilePaths(&options, []string{"--some-config", optionsFilePath}, func(s string) (os.FileInfo, error) {
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
		options.filePath = t.TempDir()
		_, err := flags.LoadWithDefaultFilePaths(&options, nil, nil)
		assert.NoError(t, err)
	})

	t.Run("when options has a nested struct", func(t *testing.T) {
		type SubOptions struct {
			SomeConfig string `long:"some-config" default:"config.txt"`
		}
		var options struct {
			testTileDir
			SubOptions
		}
		options.filePath = t.TempDir()

		someConfigPath := filepath.Join(options.filePath, "config.txt")
		f, err := os.Create(someConfigPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		var statParam string
		_, err = flags.LoadWithDefaultFilePaths(&options, nil, func(s string) (os.FileInfo, error) {
			statParam = s
			return os.Stat(s)
		})
		require.NoError(t, err)
		assert.Equal(t, someConfigPath, statParam, "it checks that the directory exists")
		assert.Equal(t, someConfigPath, options.SomeConfig, "it sets the struct field")
	})

	t.Run("when the field is a slice", func(t *testing.T) {
		t.Run("and the default files exist", func(t *testing.T) {
			var options struct {
				testTileDir
				Configurations []string `long:"configuration" default:"server.yml,client.yml,database.yml"`
			}
			dir := t.TempDir()
			options.filePath = dir

			for _, defaultFileName := range []string{"server.yml", "client.yml", "database.yml"} {
				f, err := os.Create(filepath.Join(dir, defaultFileName))
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}

			var statParams []string
			_, err := flags.LoadWithDefaultFilePaths(&options, nil, func(s string) (os.FileInfo, error) {
				statParams = append(statParams, s)
				return os.Stat(s)
			})
			require.NoError(t, err)
			assert.Equal(t, statParams, options.Configurations, "it checks if the files exist")
			assert.Equal(t, []string{filepath.Join(dir, "server.yml"), filepath.Join(dir, "client.yml"), filepath.Join(dir, "database.yml")}, options.Configurations, "it sets the struct field")
		})

		t.Run("and a default file does not exist", func(t *testing.T) {
			var options struct {
				testTileDir
				Configurations []string `long:"configuration" default:"server.yml, MISSING.yml, database.yml"`
			}
			dir := t.TempDir()
			options.filePath = dir

			for _, defaultFileName := range []string{"server.yml", "database.yml"} {
				f, err := os.Create(filepath.Join(dir, defaultFileName))
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}

			_, err := flags.LoadWithDefaultFilePaths(&options, nil, os.Stat)
			require.NoError(t, err)
			assert.Equal(t, []string{filepath.Join(dir, "server.yml"), filepath.Join(dir, "database.yml")}, options.Configurations, "it only sets the defaults that exist")
		})

		t.Run("and there is exactly one default", func(t *testing.T) {
			var options struct {
				testTileDir
				Configurations []string `long:"configuration" default:"config.yml"`
			}
			dir := t.TempDir()
			options.filePath = dir

			f, err := os.Create(filepath.Join(dir, "config.yml"))
			require.NoError(t, err)
			require.NoError(t, f.Close())

			_, err = flags.LoadWithDefaultFilePaths(&options, nil, os.Stat)
			require.NoError(t, err)
			assert.Equal(t, []string{filepath.Join(dir, "config.yml")}, options.Configurations, "it only sets the defaults if it exist")
		})

		t.Run("and arguments are passed", func(t *testing.T) {
			var options struct {
				testTileDir
				Configurations []string `long:"configuration" default:"config.yml"`
			}
			dir := t.TempDir()
			options.filePath = dir

			f, err := os.Create(filepath.Join(dir, "config.yml"))
			require.NoError(t, err)
			require.NoError(t, f.Close())

			_, err = flags.LoadWithDefaultFilePaths(&options, []string{"--configuration", "some-config.yml"}, os.Stat)
			require.NoError(t, err)
			assert.Equal(t, []string{"some-config.yml"}, options.Configurations, "it sets the field without string modification")
		})

		t.Run("and the default tag is not added", func(t *testing.T) {
			var options struct {
				testTileDir
				Configurations []string `long:"configuration"`
			}
			dir := t.TempDir()
			options.filePath = dir

			_, err := flags.LoadWithDefaultFilePaths(&options, nil, os.Stat)
			require.NoError(t, err)
		})

		t.Run("and the field type is not a string", func(t *testing.T) {
			var options struct {
				testTileDir
				N []int `long:"n"`
			}
			dir := t.TempDir()
			options.filePath = dir

			require.Panics(t, func() {
				_, _ = flags.LoadWithDefaultFilePaths(&options, nil, os.Stat)
			}, "it panics because jhanda does not permit non-string slice fields")
		})

		t.Run("and there are additional options and args are passed", func(t *testing.T) {
			// this is in response to a regression
			type AdditionalOptions struct {
				BOSHVariableDirectories []string `short:"vd"  long:"bosh-variables-directory"   default:"bosh_variables"   description:"path to a directory containing BOSH variables"`
			}
			var options struct {
				testTileDir
				AdditionalOptions
			}
			dir := t.TempDir()
			options.filePath = dir

			_, err := flags.LoadWithDefaultFilePaths(&options, []string{"--bosh-variables-directory", "some-dir", "--bosh-variables-directory", "other-dir"}, os.Stat)
			require.NoError(t, err)
			assert.Equal(t, []string{"some-dir", "other-dir"}, options.BOSHVariableDirectories, "it sets the field without string modification")
		})
	})

	t.Run("when a field is not tagged with a default value", func(t *testing.T) {
		var options struct {
			testTileDir
			SomeConfig string `long:"some-config"`
		}
		options.filePath = t.TempDir()

		var statCallCount int

		_, err := flags.LoadWithDefaultFilePaths(&options, nil, func(s string) (os.FileInfo, error) {
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
		options.filePath = t.TempDir()
		assert.Panics(t, func() {
			_, _ = flags.LoadWithDefaultFilePaths(&options, nil, nil)
		}, "jhana panics")
	})

	t.Run("when defaults are over-written for multiple fields", func(t *testing.T) {
		type Embedded struct {
			Configurations []string `long:"configuration" default:"c.yml"`
		}
		var options struct {
			testTileDir
			Embedded
			Directories []string `long:"directory"  default:"someplace"`
			Files       []string `long:"file"       default:"f1.yml, f2.yml"`
			FinalField  []string `long:"final"       default:"final.txt, end.txt"`
		}
		dir := t.TempDir()
		options.filePath = dir

		for _, defaultFileName := range []string{"final.txt", "end.txt"} {
			f, err := os.Create(filepath.Join(dir, defaultFileName))
			require.NoError(t, err)
			require.NoError(t, f.Close())
		}

		_, err := flags.LoadWithDefaultFilePaths(&options, []string{"--directory=dir1", "--directory=dir2", "--file=phil"}, os.Stat)
		assert.NoError(t, err)

		assert.Equal(t, []string{"phil"}, options.Files, "it removes defaults from other fields")
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

		assert.Equal(t, got, []string{
			"--kilnfile", "kilnfile1",
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
