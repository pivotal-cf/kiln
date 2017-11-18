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
				"--icon", "some-icon-path",
				"--metadata", "some-metadata",
				"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
				"--releases-directory", "some-release-tarball-directory",
				"--stemcell-tarball", "some-stemcell-tarball",
				"--version", "1.2.3",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(tileMaker.MakeCallCount()).To(Equal(1))

			config := tileMaker.MakeArgsForCall(0)
			Expect(config).To(Equal(commands.BakeConfig{
				IconPath:           "some-icon-path",
				Metadata:           "some-metadata",
				OutputFile:         "some-output-dir/cool-product-file-1.2.3-build.4",
				ReleaseDirectories: []string{"some-release-tarball-directory"},
				StemcellTarball:    "some-stemcell-tarball",
				Version:            "1.2.3",
			}))
		})

		Context("when there are multiple migrations directories", func() {
			It("builds the tile", func() {
				err := bake.Execute([]string{
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--migrations-directory", "some-migrations-directory",
					"--migrations-directory", "some-other-migrations-directory",
					"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
					"--releases-directory", "some-release-tarball-directory",
					"--stemcell-tarball", "some-stemcell-tarball",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(tileMaker.MakeCallCount()).To(Equal(1))

				config := tileMaker.MakeArgsForCall(0)
				Expect(config).To(Equal(commands.BakeConfig{
					IconPath:             "some-icon-path",
					Metadata:             "some-metadata",
					MigrationDirectories: []string{"some-migrations-directory", "some-other-migrations-directory"},
					OutputFile:           "some-output-dir/cool-product-file-1.2.3-build.4",
					ReleaseDirectories:   []string{"some-release-tarball-directory"},
					StemcellTarball:      "some-stemcell-tarball",
					Version:              "1.2.3",
				}))
			})
		})

		Context("when there are multiple release directories", func() {
			It("builds the tile", func() {
				err := bake.Execute([]string{
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
					"--releases-directory", "other-release-tarball-directory",
					"--releases-directory", "some-release-tarball-directory",
					"--stemcell-tarball", "some-stemcell-tarball",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(tileMaker.MakeCallCount()).To(Equal(1))

				config := tileMaker.MakeArgsForCall(0)
				Expect(config).To(Equal(commands.BakeConfig{
					IconPath:           "some-icon-path",
					StemcellTarball:    "some-stemcell-tarball",
					ReleaseDirectories: []string{"other-release-tarball-directory", "some-release-tarball-directory"},
					Metadata:           "some-metadata",
					Version:            "1.2.3",
					OutputFile:         "some-output-dir/cool-product-file-1.2.3-build.4",
				}))
			})
		})

		Context("when there are multiple runtime-configs directories", func() {
			It("builds the tile", func() {
				err := bake.Execute([]string{
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
					"--releases-directory", "some-release-tarball-directory",
					"--runtime-configs-directory", "some-other-runtime-configs-directory",
					"--runtime-configs-directory", "some-runtime-configs-directory",
					"--stemcell-tarball", "some-stemcell-tarball",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(tileMaker.MakeCallCount()).To(Equal(1))

				config := tileMaker.MakeArgsForCall(0)
				Expect(config).To(Equal(commands.BakeConfig{
					IconPath:                 "some-icon-path",
					Metadata:                 "some-metadata",
					OutputFile:               "some-output-dir/cool-product-file-1.2.3-build.4",
					ReleaseDirectories:       []string{"some-release-tarball-directory"},
					RuntimeConfigDirectories: []string{"some-other-runtime-configs-directory", "some-runtime-configs-directory"},
					StemcellTarball:          "some-stemcell-tarball",
					Version:                  "1.2.3",
				}))
			})
		})

		Context("when there are multiple variables directories", func() {
			It("builds the tile", func() {
				err := bake.Execute([]string{
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
					"--releases-directory", "some-release-tarball-directory",
					"--stemcell-tarball", "some-stemcell-tarball",
					"--variables-directory", "some-other-variables-directory",
					"--variables-directory", "some-variables-directory",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(tileMaker.MakeCallCount()).To(Equal(1))

				config := tileMaker.MakeArgsForCall(0)
				Expect(config).To(Equal(commands.BakeConfig{
					IconPath:            "some-icon-path",
					StemcellTarball:     "some-stemcell-tarball",
					ReleaseDirectories:  []string{"some-release-tarball-directory"},
					VariableDirectories: []string{"some-other-variables-directory", "some-variables-directory"},
					Metadata:            "some-metadata",
					Version:             "1.2.3",
					OutputFile:          "some-output-dir/cool-product-file-1.2.3-build.4",
				}))
			})
		})

		Context("when files to embed are specified", func() {
			It("builds the tile", func() {
				err := bake.Execute([]string{
					"--embed", "some-file-to-embed",
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
					"--releases-directory", "some-release-tarball-directory",
					"--stemcell-tarball", "some-stemcell-tarball",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(tileMaker.MakeCallCount()).To(Equal(1))

				config := tileMaker.MakeArgsForCall(0)
				Expect(config).To(Equal(commands.BakeConfig{
					EmbedPaths:         []string{"some-file-to-embed"},
					IconPath:           "some-icon-path",
					Metadata:           "some-metadata",
					OutputFile:         "some-output-dir/cool-product-file-1.2.3-build.4",
					ReleaseDirectories: []string{"some-release-tarball-directory"},
					StemcellTarball:    "some-stemcell-tarball",
					Version:            "1.2.3",
				}))
			})
		})

		Context("failure cases", func() {
			Context("when the release-tarball flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--stemcell-tarball", "some-stemcell-tarball",
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--version", "1.2.3",
						"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
						"--stub-releases",
					})

					Expect(err).To(MatchError("Please specify release tarballs directory with the --releases-directory parameter"))
				})
			})

			Context("when the stemcell-tarball flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--releases-directory", "some-release-tarball-directory",
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--version", "1.2.3",
						"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--stemcell-tarball is a required parameter"))
				})
			})

			Context("when the icon flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
						"--releases-directory", "some-release-tarball-directory",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--stub-releases",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--icon is a required parameter"))
				})
			})

			Context("when the metadata flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
						"--releases-directory", "some-release-tarball-directory",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--stub-releases",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--metadata is a required parameter"))
				})
			})

			Context("when the version flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/cool-product-file-1.2.3-build.4",
						"--releases-directory", "some-release-tarball-directory",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--stub-releases",
					})

					Expect(err).To(MatchError("--version is a required parameter"))
				})
			})

			Context("when the output-file flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--releases-directory", "some-release-tarball-directory",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--stub-releases",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--output-file is a required parameter"))
				})
			})
		})
	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			command := commands.NewBake(nil)
			Expect(command.Usage()).To(Equal(jhandacommands.Usage{
				Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
				ShortDescription: "bakes a tile",
				Flags:            command.Options,
			}))
		})
	})
})
