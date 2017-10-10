package commands_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	jhandacommands "github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
)

var _ = Describe("bake", func() {
	var (
		tileMaker *fakes.TileMaker
		bake      commands.Bake
	)

	BeforeEach(func() {
		tileMaker = &fakes.TileMaker{}
		bake = commands.NewBake(tileMaker)
	})

	Describe("Execute", func() {
		It("builds the tile", func() {
			err := bake.Execute([]string{
				"--stemcell-tarball", "some-stemcell-tarball",
				"--release-tarball", "some-release-tarball",
				"--release-tarball", "some-other-release-tarball",
				"--handcraft", "some-handcraft",
				"--version", "1.2.3-build.4",
				"--final-version", "1.2.3",
				"--product-name", "cool-product-name",
				"--filename-prefix", "cool-product-file",
				"--output-dir", "some-output-dir",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(tileMaker.MakeCallCount()).To(Equal(1))

			config := tileMaker.MakeArgsForCall(0)
			Expect(config).To(Equal(commands.BakeConfig{
				StemcellTarball: "some-stemcell-tarball",
				ReleaseTarballs: []string{"some-release-tarball", "some-other-release-tarball"},
				Handcraft:       "some-handcraft",
				Version:         "1.2.3-build.4",
				FinalVersion:    "1.2.3",
				ProductName:     "cool-product-name",
				FilenamePrefix:  "cool-product-file",
				OutputDir:       "some-output-dir",
			}))
		})

		Context("when migrations directory is provided", func() {
			It("builds the tile", func() {
				err := bake.Execute([]string{
					"--stemcell-tarball", "some-stemcell-tarball",
					"--release-tarball", "some-release-tarball",
					"--release-tarball", "some-other-release-tarball",
					"--migrations-directory", "some-migrations-directory",
					"--handcraft", "some-handcraft",
					"--version", "1.2.3-build.4",
					"--final-version", "1.2.3",
					"--product-name", "cool-product-name",
					"--filename-prefix", "cool-product-file",
					"--output-dir", "some-output-dir",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(tileMaker.MakeCallCount()).To(Equal(1))

				config := tileMaker.MakeArgsForCall(0)
				Expect(config).To(Equal(commands.BakeConfig{
					StemcellTarball:      "some-stemcell-tarball",
					ReleaseTarballs:      []string{"some-release-tarball", "some-other-release-tarball"},
					Handcraft:            "some-handcraft",
					Version:              "1.2.3-build.4",
					FinalVersion:         "1.2.3",
					ProductName:          "cool-product-name",
					FilenamePrefix:       "cool-product-file",
					OutputDir:            "some-output-dir",
					MigrationDirectories: []string{"some-migrations-directory"},
				}))
			})

			Context("when the migration directory is specified multiple times", func() {
				It("builds the tile", func() {
					err := bake.Execute([]string{
						"--stemcell-tarball", "some-stemcell-tarball",
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migrations-directory", "some-migrations-directory",
						"--migrations-directory", "some-other-migrations-directory",
						"--handcraft", "some-handcraft",
						"--version", "1.2.3-build.4",
						"--final-version", "1.2.3",
						"--product-name", "cool-product-name",
						"--filename-prefix", "cool-product-file",
						"--output-dir", "some-output-dir",
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(tileMaker.MakeCallCount()).To(Equal(1))

					config := tileMaker.MakeArgsForCall(0)
					Expect(config).To(Equal(commands.BakeConfig{
						StemcellTarball:      "some-stemcell-tarball",
						ReleaseTarballs:      []string{"some-release-tarball", "some-other-release-tarball"},
						Handcraft:            "some-handcraft",
						Version:              "1.2.3-build.4",
						FinalVersion:         "1.2.3",
						ProductName:          "cool-product-name",
						FilenamePrefix:       "cool-product-file",
						OutputDir:            "some-output-dir",
						MigrationDirectories: []string{"some-migrations-directory", "some-other-migrations-directory"},
					}))

				})

			})
		})

		It("builds the tile", func() {
			err := bake.Execute([]string{
				"--stemcell-tarball", "some-stemcell-tarball",
				"--release-tarball", "some-release-tarball",
				"--release-tarball", "some-other-release-tarball",
				"--handcraft", "some-handcraft",
				"--version", "1.2.3-build.4",
				"--final-version", "1.2.3",
				"--product-name", "cool-product-name",
				"--filename-prefix", "cool-product-file",
				"--output-dir", "some-output-dir",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(tileMaker.MakeCallCount()).To(Equal(1))

			config := tileMaker.MakeArgsForCall(0)
			Expect(config).To(Equal(commands.BakeConfig{
				StemcellTarball: "some-stemcell-tarball",
				ReleaseTarballs: []string{"some-release-tarball", "some-other-release-tarball"},
				Handcraft:       "some-handcraft",
				Version:         "1.2.3-build.4",
				FinalVersion:    "1.2.3",
				ProductName:     "cool-product-name",
				FilenamePrefix:  "cool-product-file",
				OutputDir:       "some-output-dir",
			}))
		})

		Context("failure cases", func() {
			Context("when content migrations are provided", func() {
				It("returns an error when base content migration is not provided", func() {
					err := bake.Execute([]string{
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

			Context("when the release-tarball flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migrations-directory", "some-migrations-directory",
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
					err := bake.Execute([]string{
						"--release-tarball", "some-release-tarball",
						"--release-tarball", "some-other-release-tarball",
						"--migrations-directory", "some-migrations-directory",
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

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			command := commands.NewBake(nil)
			Expect(command.Usage()).To(Equal(jhandacommands.Usage{
				Description:      "Builds a tile to be uploaded to OpsMan from provided inputs.",
				ShortDescription: "builds a tile",
				Flags:            command.Options,
			}))
		})
	})
})
