package kiln_test

import (
	"github.com/pivotal-cf/kiln/kiln"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("arg parser", func() {
	var (
		argParser kiln.ArgParser
	)

	Describe("Parse", func() {
		BeforeEach(func() {
			argParser = kiln.NewArgParser()
		})

		It("parses cli args into a config", func() {
			config, err := argParser.Parse([]string{
				"--release-tarball", "some-release-tarball",
				"--release-tarball", "some-other-release-tarball",
				"--migration", "some-migration",
				"--migration", "some-other-migration",
				"--stemcell-tarball", "some-stemcell-tarball",
				"--handcraft", "some-handcraft",
				"--version", "1.2.3-build.4",
				"--final-version", "1.2.3",
				"--product-name", "cool-product-name",
				"--filename-prefix", "cool-product-file",
				"--output-dir", "some-output-dir",
				"--stub-releases",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(config).To(Equal(kiln.ApplicationConfig{
				ReleaseTarballs: []string{"some-release-tarball", "some-other-release-tarball"},
				Migrations:      []string{"some-migration", "some-other-migration"},
				StemcellTarball: "some-stemcell-tarball",
				Handcraft:       "some-handcraft",
				Version:         "1.2.3-build.4",
				FinalVersion:    "1.2.3",
				ProductName:     "cool-product-name",
				FilenamePrefix:  "cool-product-file",
				OutputDir:       "some-output-dir",
				StubReleases:    true,
			}))
		})

		Context("when content migrations are provided", func() {
			It("parses cli args into a config", func() {
				config, err := argParser.Parse([]string{
					"--release-tarball", "some-release-tarball",
					"--release-tarball", "some-other-release-tarball",
					"--content-migration", "some-migration",
					"--content-migration", "some-other-migration",
					"--base-content-migration", "some-base-content-migration",
					"--stemcell-tarball", "some-stemcell-tarball",
					"--handcraft", "some-handcraft",
					"--version", "1.2.3-build.4",
					"--final-version", "1.2.3",
					"--product-name", "cool-product-name",
					"--filename-prefix", "cool-product-file",
					"--output-dir", "some-output-dir",
					"--stub-releases",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(config).To(Equal(kiln.ApplicationConfig{
					ReleaseTarballs:      []string{"some-release-tarball", "some-other-release-tarball"},
					BaseContentMigration: "some-base-content-migration",
					ContentMigrations:    []string{"some-migration", "some-other-migration"},
					StemcellTarball:      "some-stemcell-tarball",
					Handcraft:            "some-handcraft",
					Version:              "1.2.3-build.4",
					FinalVersion:         "1.2.3",
					ProductName:          "cool-product-name",
					FilenamePrefix:       "cool-product-file",
					OutputDir:            "some-output-dir",
					StubReleases:         true,
				}))
			})

			Context("error handling", func() {
				It("returns an error when base content migration is not provided", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--content-migration", "some-migration",
						"--content-migration", "some-other-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})
					Expect(err).To(MatchError("base content migration is required when content migrations are provided"))
				})
			})
		})

		Context("error handling", func() {
			Context("when the release-tarball flag is missing", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})

					Expect(err).To(MatchError("Please specify at least one release tarball with the --release-tarball parameter"))
				})
			})

			Context("when the stemcell-tarball flag is missing", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--stemcell-tarball is a required parameter"))
				})
			})

			Context("when the handcraft flag is missing", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--handcraft is a required parameter"))
				})
			})

			Context("when the version flag is missing", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--version is a required parameter"))
				})
			})

			Context("when the final-version flag is missing", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--final-version is a required parameter"))
				})
			})

			Context("when the product-name flag is missing", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--product-name is a required parameter"))
				})
			})

			Context("when the filename-prefix flag is missing", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--filename-prefix is a required parameter"))
				})
			})

			Context("when the output-dir is missing", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--output-dir is a required parameter"))
				})
			})

			Context("when content migrations and migrations are provided", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--content-migration", "some-content-migration",
						"--content-migration", "some-other-content-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})
					Expect(err).To(MatchError("cannot build a tile with content migrations and migrations"))
				})
			})

			Context("when base content migrations and migrations are provided", func() {
				It("returns an error", func() {
					_, err := argParser.Parse([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migration", "some-migration",
						"--migration", "some-other-migration",
						"--base-content-migration", "some-base-content-migration",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
						"--stub-releases",
					})
					Expect(err).To(MatchError("cannot build a tile with a base content migration and migrations"))
				})
			})
		})
	})
})
