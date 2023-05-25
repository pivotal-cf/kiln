package flags_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"strings"
)

var _ = Describe("ToStrings", func() {
	Context("when booleans are true", func() {
		It("encodes an options struct into a string slice with jhanda formatting", func() {
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

			jhandaArguments := flags.ToStrings(options)

			expectedJhandaArguments := "--kilnfile kilnfile1 " +
				"--variables-file variables-files-1 " +
				"--variables-file variables-files-2 " +
				"--variable variables-1 --variable variables-2 " +
				"--download-threads 0 " +
				"--no-confirm " +
				"--releases-directory releases-dir"

			Expect(jhandaArguments).To(Equal(strings.Split(expectedJhandaArguments, " ")))
		})
	})
	Context("when booleans are false", func() {
		It("encodes an options struct into a string slice with jhanda formatting", func() {
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

			jhandaArguments := flags.ToStrings(options)

			expectedJhandaArguments := "--kilnfile kilnfile1 " +
				"--variables-file variables-files-1 " +
				"--variables-file variables-files-2 " +
				"--variable variables-1 --variable variables-2 " +
				"--download-threads 0 " +
				"--releases-directory releases-dir"

			Expect(jhandaArguments).To(Equal(strings.Split(expectedJhandaArguments, " ")))
		})
	})
})
