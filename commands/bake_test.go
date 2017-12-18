package commands_test

import (
	"errors"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"

	jhandacommands "github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
)

var _ = Describe("bake", func() {
	var (
		fakeMetadataBuilder        *fakes.MetadataBuilder
		fakeReleaseManifestReader  *fakes.ReleaseManifestReader
		fakeStemcellManifestReader *fakes.StemcellManifestReader
		fakeFormDirectoryReader    *fakes.FormDirectoryReader
		fakeInterpolator           *fakes.Interpolator
		fakeTileWriter             *fakes.TileWriter
		fakeLogger                 *fakes.Logger

		generatedMetadata      builder.GeneratedMetadata
		otherReleasesDirectory string
		otherTarballRelease    string
		someReleasesDirectory  string
		tarballRelease         string
		tmpDir                 string
		variableFile           *os.File

		bake commands.Bake
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "command-test")
		Expect(err).NotTo(HaveOccurred())

		variableFile, err = ioutil.TempFile(tmpDir, "variables-file")
		Expect(err).NotTo(HaveOccurred())

		variables := map[string]string{"some-variable-from-file": "some-variable-value-from-file"}
		data, err := yaml.Marshal(&variables)
		Expect(err).NotTo(HaveOccurred())

		n, err := variableFile.Write(data)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(HaveLen(n))

		someReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
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
		fakeReleaseManifestReader = &fakes.ReleaseManifestReader{}
		fakeStemcellManifestReader = &fakes.StemcellManifestReader{}
		fakeFormDirectoryReader = &fakes.FormDirectoryReader{}
		fakeInterpolator = &fakes.Interpolator{}
		fakeTileWriter = &fakes.TileWriter{}
		fakeLogger = &fakes.Logger{}

		fakeReleaseManifestReader.ReadReturnsOnCall(0, builder.ReleaseManifest{
			Name:    "some-release-1",
			Version: "1.2.3",
			File:    "release1.tgz",
		}, nil)

		fakeReleaseManifestReader.ReadReturnsOnCall(1, builder.ReleaseManifest{
			Name:    "some-release-2",
			Version: "2.3.4",
			File:    "release2.tgz",
		}, nil)

		fakeStemcellManifestReader.ReadReturns(builder.StemcellManifest{
			Version:         "2.3.4",
			OperatingSystem: "an-operating-system",
		}, nil)

		fakeFormDirectoryReader.ReadReturns([]builder.Part{
			{
				Name: "some-form",
				Metadata: builder.Metadata{
					"name":  "some-form",
					"label": "some-form-label",
				},
			},
		}, nil)

		generatedMetadata = builder.GeneratedMetadata{
			IconImage: "some-icon-image",
			Name:      "some-product-name",
			StemcellCriteria: builder.StemcellCriteria{
				Version: "2.3.4",
				OS:      "an-operating-system",
			},
		}
		fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)
		fakeInterpolator.InterpolateReturns([]byte("some-interpolated-metadata"), nil)

		bake = commands.NewBake(
			fakeMetadataBuilder,
			fakeInterpolator,
			fakeTileWriter,
			fakeLogger,
			fakeReleaseManifestReader,
			fakeStemcellManifestReader,
			fakeFormDirectoryReader,
		)
	})

	AfterEach(func() {
		Expect(variableFile.Close()).To(Succeed())
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Execute", func() {
		It("builds the tile", func() {

			err := bake.Execute([]string{
				"--embed", "some-embed-path",
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
				"--migrations-directory", "some-migrations-directory",
				"--migrations-directory", "some-other-migrations-directory",
				"--variable", "some-variable=some-variable-value",
				"--variables-file", variableFile.Name(),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetadataBuilder.BuildCallCount()).To(Equal(1))

			Expect(fakeInterpolator.InterpolateCallCount()).To(Equal(1))
			Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))
			Expect(fakeFormDirectoryReader.ReadCallCount()).To(Equal(1))
		})

		Context("when the optional flags are not specified", func() {
			It("builds the metadata", func() {
				err := bake.Execute([]string{
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--releases-directory", someReleasesDirectory,
					"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when multiple variable files are provided", func() {
			var otherVariableFile *os.File

			BeforeEach(func() {
				var err error
				otherVariableFile, err = ioutil.TempFile(tmpDir, "variables-file")
				Expect(err).NotTo(HaveOccurred())
				defer otherVariableFile.Close()

				variables := map[string]string{
					"some-variable-from-file":       "override-variable-from-other-file",
					"some-other-variable-from-file": "some-other-variable-value-from-file",
				}
				data, err := yaml.Marshal(&variables)
				Expect(err).NotTo(HaveOccurred())

				n, err := otherVariableFile.Write(data)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(HaveLen(n))
			})

			It("interpolates variables from both files", func() {
				generatedMetadata.Metadata = builder.Metadata{
					"custom_variable":               "$(variable \"some-variable\")",
					"variable_from_file":            "$(variable \"some-variable-from-file\")",
					"some_other_variable_from_file": "$(variable \"some-other-variable-from-file\")",
					"icon_image":                    "$( icon )",
					"releases":                      []string{"$(release \"some-release-1\")"},
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
					"--variables-file", variableFile.Name(),
					"--variables-file", otherVariableFile.Name(),
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())

				_, generatedMetadataContents, _ := fakeTileWriter.WriteArgsForCall(0)
				Expect(generatedMetadataContents).To(MatchYAML("some-interpolated-metadata"))
			})
		})

		Context("failure cases", func() {
			Context("when the variable file does not exist but is provided", func() {
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
						"--variables-file", "bogus",
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("no such file or directory"))
				})
			})

			Context("when the release manifest reader returns an error", func() {
				It("returns an error", func() {
					fakeReleaseManifestReader.ReadReturnsOnCall(0, builder.ReleaseManifest{}, errors.New("some-error"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("some-error"))
				})
			})

			Context("when the stemcell manifest reader returns an error", func() {
				It("returns an error", func() {
					fakeStemcellManifestReader.ReadReturns(builder.StemcellManifest{}, errors.New("some-error"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("some-error"))
				})
			})

			Context("when the form directory reader returns an error", func() {
				It("returns an error", func() {
					fakeFormDirectoryReader.ReadReturns(nil, errors.New("some-error"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--forms-directory", "some-form-directory",
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("some-error"))
				})
			})

			Context("when the template interpolator returns an error", func() {
				It("returns the error", func() {
					fakeInterpolator.InterpolateReturns(nil, errors.New("some-error"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--forms-directory", "some-form-directory",
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("some-error"))
				})
			})

			Context("when the variable file does not contain valid YAML", func() {
				BeforeEach(func() {
					_, err := variableFile.Write([]byte("{{invalid-blah"))
					Expect(err).NotTo(HaveOccurred())
				})

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
						"--variables-file", variableFile.Name(),
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("failed reading variable file:"))
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

			Context("when an invalid flag is passed", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--jobs-directory", "some-jobs-directory",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
						"--non-existant-flag",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("non-existant-flag"))
				})
			})
		})
	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			command := commands.NewBake(fakeMetadataBuilder, fakeInterpolator, fakeTileWriter, fakeLogger, fakeReleaseManifestReader, fakeStemcellManifestReader, fakeFormDirectoryReader)
			Expect(command.Usage()).To(Equal(jhandacommands.Usage{
				Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
				ShortDescription: "bakes a tile",
				Flags:            command.Options,
			}))
		})
	})
})
