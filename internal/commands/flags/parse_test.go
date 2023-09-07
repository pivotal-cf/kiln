package flags_test

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

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
