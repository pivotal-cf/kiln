package commands_test

import (
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	jhandacommands "github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
)

var _ = Describe("bake", func() {
	var (
		fakeMetadataBuilder *fakes.MetadataBuilder
		fakeTileWriter      *fakes.TileWriter
		fakeLogger          *fakes.Logger

		generatedMetadata      builder.GeneratedMetadata
		someReleasesDirectory  string
		otherReleasesDirectory string
		tarballRelease         string
		otherTarballRelease    string
		err                    error

		bake commands.Bake
	)

	BeforeEach(func() {
		someReleasesDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		tarballRelease = someReleasesDirectory + "/release1.tgz"
		err = ioutil.WriteFile(tarballRelease, []byte(""), 0644)
		Expect(err).NotTo(HaveOccurred())

		otherTarballRelease = otherReleasesDirectory + "/release2.tgz"
		err = ioutil.WriteFile(otherTarballRelease, []byte(""), 0644)
		Expect(err).NotTo(HaveOccurred())

		nonTarballRelease := someReleasesDirectory + "/some-broken-release"
		err = ioutil.WriteFile(nonTarballRelease, []byte(""), 0644)
		Expect(err).NotTo(HaveOccurred())

		fakeMetadataBuilder = &fakes.MetadataBuilder{}
		fakeTileWriter = &fakes.TileWriter{}
		fakeLogger = &fakes.Logger{}

		generatedMetadata = builder.GeneratedMetadata{
			IconImage: "some-icon-image",
			Name:      "some-product-name",
			Releases: []builder.Release{{
				Name:    "some-release",
				File:    "some-release-tarball",
				Version: "1.2.3-build.4",
			}},
			StemcellCriteria: builder.StemcellCriteria{
				Version:     "2.3.4",
				OS:          "an-operating-system",
				RequiresCPI: false,
			},
		}
		fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

		bake = commands.NewBake(fakeMetadataBuilder, fakeTileWriter, fakeLogger)
	})

	Describe("Execute", func() {
		It("builds the metadata", func() {
			err := bake.Execute([]string{
				"--forms-directory", "some-forms-directory",
				"--icon", "some-icon-path",
				"--instance-groups-directory", "some-instance-groups-directory",
				"--jobs-directory", "some-jobs-directory",
				"--metadata", "some-metadata",
				"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
				"--properties-directory", "some-properties-directory",
				"--releases-directory", otherReleasesDirectory,
				"--releases-directory", someReleasesDirectory,
				"--runtime-configs-directory", "some-other-runtime-configs-directory",
				"--runtime-configs-directory", "some-runtime-configs-directory",
				"--stemcell-tarball", "some-stemcell-tarball",
				"--bosh-variables-directory", "some-other-variables-directory",
				"--bosh-variables-directory", "some-variables-directory",
				"--version", "1.2.3",
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetadataBuilder.BuildCallCount()).To(Equal(1))

			buildInput := fakeMetadataBuilder.BuildArgsForCall(0)
			Expect(buildInput.FormDirectories).To(Equal([]string{"some-forms-directory"}))
			Expect(buildInput.IconPath).To(Equal("some-icon-path"))
			Expect(buildInput.InstanceGroupDirectories).To(Equal([]string{"some-instance-groups-directory"}))
			Expect(buildInput.JobDirectories).To(Equal([]string{"some-jobs-directory"}))
			Expect(buildInput.MetadataPath).To(Equal("some-metadata"))
			Expect(buildInput.PropertyDirectories).To(Equal([]string{"some-properties-directory"}))
			Expect(buildInput.ReleaseTarballs).To(Equal([]string{otherTarballRelease, tarballRelease}))
			Expect(buildInput.RuntimeConfigDirectories).To(Equal([]string{"some-other-runtime-configs-directory", "some-runtime-configs-directory"}))
			Expect(buildInput.StemcellTarball).To(Equal("some-stemcell-tarball"))
			Expect(buildInput.BOSHVariableDirectories).To(Equal([]string{"some-other-variables-directory", "some-variables-directory"}))
			Expect(buildInput.Version).To(Equal("1.2.3"))
		})

		It("calls the tile writer", func() {
			generatedMetadata.Metadata = builder.Metadata{
				"custom_variable": "$(variable \"some-variable\")",
			}
			fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

			err := bake.Execute([]string{
				"--embed", "some-embed-path",
				"--forms-directory", "some-forms-directory",
				"--icon", "some-icon-path",
				"--instance-groups-directory", "some-instance-groups-directory",
				"--jobs-directory", "some-jobs-directory",
				"--metadata", "some-metadata",
				"--migrations-directory", "some-migrations-directory",
				"--migrations-directory", "some-other-migrations-directory",
				"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
				"--releases-directory", otherReleasesDirectory,
				"--releases-directory", someReleasesDirectory,
				"--runtime-configs-directory", "some-runtime-configs-directory",
				"--stemcell-tarball", "some-stemcell-tarball",
				"--bosh-variables-directory", "some-variables-directory",
				"--variable", "some-variable=some-variable-value",
				"--version", "1.2.3",
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))

			productName, generatedMetadataContents, actualConfig := fakeTileWriter.WriteArgsForCall(0)
			Expect(productName).To(Equal("some-product-name"))
			Expect(generatedMetadataContents).To(MatchYAML(`
icon_image: some-icon-image
name: some-product-name
custom_variable: some-variable-value
releases:
- name: some-release
  file: some-release-tarball
  version: 1.2.3-build.4
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
  requires_cpi: false`))
			Expect(actualConfig).To(Equal(builder.WriteInput{
				EmbedPaths:           []string{"some-embed-path"},
				MigrationDirectories: []string{"some-migrations-directory", "some-other-migrations-directory"},
				OutputFile:           "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
				ReleaseDirectories:   []string{otherReleasesDirectory, someReleasesDirectory},
			}))
		})

		It("logs its step", func() {
			err := bake.Execute([]string{
				"--forms-directory", "some-forms-directory",
				"--icon", "some-icon-path",
				"--instance-groups-directory", "some-instance-groups-directory",
				"--jobs-directory", "some-jobs-directory",
				"--metadata", "some-metadata",
				"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
				"--releases-directory", otherReleasesDirectory,
				"--releases-directory", someReleasesDirectory,
				"--runtime-configs-directory", "some-runtime-configs-directory",
				"--stemcell-tarball", "some-stemcell-tarball",
				"--bosh-variables-directory", "some-variables-directory",
				"--version", "1.2.3",
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLogger.PrintlnCallCount()).To(Equal(1))

			logLines := fakeLogger.PrintlnArgsForCall(0)

			Expect(logLines[0]).To(Equal("Marshaling metadata file..."))
		})

		Context("when the optional flags are not specified", func() {
			It("builds the metadata", func() {
				err := bake.Execute([]string{
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
					"--releases-directory", someReleasesDirectory,
					"--stemcell-tarball", "some-stemcell-tarball",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())

				Expect(fakeMetadataBuilder.BuildCallCount()).To(Equal(1))

				buildInput := fakeMetadataBuilder.BuildArgsForCall(0)
				Expect(buildInput.FormDirectories).To(BeEmpty())
				Expect(buildInput.IconPath).To(Equal("some-icon-path"))
				Expect(buildInput.InstanceGroupDirectories).To(BeEmpty())
				Expect(buildInput.JobDirectories).To(BeEmpty())
				Expect(buildInput.MetadataPath).To(Equal("some-metadata"))
				Expect(buildInput.ReleaseTarballs).To(Equal([]string{tarballRelease}))
				Expect(buildInput.RuntimeConfigDirectories).To(BeEmpty())
				Expect(buildInput.StemcellTarball).To(Equal("some-stemcell-tarball"))
				Expect(buildInput.BOSHVariableDirectories).To(BeEmpty())
				Expect(buildInput.Version).To(Equal("1.2.3"))
			})

			It("calls the tile writer", func() {
				err := bake.Execute([]string{
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
					"--releases-directory", someReleasesDirectory,
					"--stemcell-tarball", "some-stemcell-tarball",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())

				Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))

				productName, generatedMetadataContents, actualConfig := fakeTileWriter.WriteArgsForCall(0)
				Expect(productName).To(Equal("some-product-name"))
				Expect(generatedMetadataContents).To(MatchYAML(`
icon_image: some-icon-image
name: some-product-name
releases:
- name: some-release
  file: some-release-tarball
  version: 1.2.3-build.4
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
  requires_cpi: false`))
				Expect(actualConfig).To(Equal(builder.WriteInput{
					OutputFile:         "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
					ReleaseDirectories: []string{someReleasesDirectory},
				}))
			})
		})

		Context("failure cases", func() {
			Context("when template parsing fails", func() {
				It("returns an error", func() {
					generatedMetadata.Metadata = builder.Metadata{
						"custom_variable": "$(variable",
					}
					fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

					err := bake.Execute([]string{
						"--forms-directory", "some-forms-directory",
						"--icon", "some-icon-path",
						"--instance-groups-directory", "some-instance-groups-directory",
						"--jobs-directory", "some-jobs-directory",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", otherReleasesDirectory,
						"--releases-directory", someReleasesDirectory,
						"--runtime-configs-directory", "some-other-runtime-configs-directory",
						"--runtime-configs-directory", "some-runtime-configs-directory",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--bosh-variables-directory", "some-other-variables-directory",
						"--bosh-variables-directory", "some-variables-directory",
						"--variable", "some-variable=some-value",
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("template parsing failed"))
				})
			})

			Context("when template execution fails", func() {
				It("returns an error", func() {
					generatedMetadata.Metadata = builder.Metadata{
						"custom_variable": "$(variable \"blah\")",
					}
					fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

					err := bake.Execute([]string{
						"--forms-directory", "some-forms-directory",
						"--icon", "some-icon-path",
						"--instance-groups-directory", "some-instance-groups-directory",
						"--jobs-directory", "some-jobs-directory",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", otherReleasesDirectory,
						"--releases-directory", someReleasesDirectory,
						"--runtime-configs-directory", "some-other-runtime-configs-directory",
						"--runtime-configs-directory", "some-runtime-configs-directory",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--bosh-variables-directory", "some-other-variables-directory",
						"--bosh-variables-directory", "some-variables-directory",
						"--variable", "some-variable=some-value",
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("template execution failed"))
					Expect(err.Error()).To(ContainSubstring("could not find variable with key"))
				})

			})

			Context("when the variable flag contains variable without equal sign", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--forms-directory", "some-forms-directory",
						"--icon", "some-icon-path",
						"--instance-groups-directory", "some-instance-groups-directory",
						"--jobs-directory", "some-jobs-directory",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", otherReleasesDirectory,
						"--releases-directory", someReleasesDirectory,
						"--runtime-configs-directory", "some-other-runtime-configs-directory",
						"--runtime-configs-directory", "some-runtime-configs-directory",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--bosh-variables-directory", "some-other-variables-directory",
						"--bosh-variables-directory", "some-variables-directory",
						"--variable", "some-variable",
						"--version", "1.2.3",
					})
					Expect(err).To(MatchError("variable needs a key value in the form of key=value"))
				})
			})

			Context("when the icon flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--icon is a required parameter"))
				})
			})

			Context("when the metadata flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--metadata is a required parameter"))
				})
			})

			Context("when the release-tarball flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("Please specify release tarballs directory with the --releases-directory parameter"))
				})
			})

			Context("when the stemcell-tarball flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
						"--releases-directory", someReleasesDirectory,
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--stemcell-tarball is a required parameter"))
				})
			})

			Context("when the version flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
					})

					Expect(err).To(MatchError("--version is a required parameter"))
				})
			})

			Context("when the output-file flag is missing", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--output-file is a required parameter"))
				})
			})

			Context("when the jobs-directory flag is passed without the instance-groups-directory flag", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--jobs-directory", "some-jobs-directory",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--jobs-directory flag requires --instance-groups-directory to also be specified"))
				})
			})
		})
	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			command := commands.NewBake(fakeMetadataBuilder, fakeTileWriter, fakeLogger)
			Expect(command.Usage()).To(Equal(jhandacommands.Usage{
				Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
				ShortDescription: "bakes a tile",
				Flags:            command.Options,
			}))
		})
	})
})
