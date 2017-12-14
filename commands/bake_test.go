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

		bake = commands.NewBake(fakeMetadataBuilder, fakeTileWriter, fakeLogger, fakeReleaseManifestReader, fakeStemcellManifestReader, fakeFormDirectoryReader)
	})

	AfterEach(func() {
		Expect(variableFile.Close()).To(Succeed())
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
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
			Expect(buildInput.IconPath).To(Equal("some-icon-path"))
			Expect(buildInput.InstanceGroupDirectories).To(Equal([]string{"some-instance-groups-directory"}))
			Expect(buildInput.JobDirectories).To(Equal([]string{"some-jobs-directory"}))
			Expect(buildInput.MetadataPath).To(Equal("some-metadata"))
			Expect(buildInput.PropertyDirectories).To(Equal([]string{"some-properties-directory"}))
			Expect(buildInput.RuntimeConfigDirectories).To(Equal([]string{"some-other-runtime-configs-directory", "some-runtime-configs-directory"}))
			Expect(buildInput.StemcellTarball).To(Equal("some-stemcell-tarball"))
			Expect(buildInput.BOSHVariableDirectories).To(Equal([]string{"some-other-variables-directory", "some-variables-directory"}))
			Expect(buildInput.Version).To(Equal("1.2.3"))
		})

		It("calls the tile writer", func() {
			generatedMetadata.Metadata = builder.Metadata{
				"custom_variable":    "$(variable \"some-variable\")",
				"variable_from_file": "$(variable \"some-variable-from-file\")",
				"releases":           []string{"$(release \"some-release-1\")", "$(release \"some-release-2\")"},
				"stemcell_criteria":  "$( stemcell )",
				"form_types":         []string{`$( form "some-form" )`},
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
				"--version", "1.2.3",
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))
			Expect(fakeFormDirectoryReader.ReadCallCount()).To(Equal(1))
			formReadArgs := fakeFormDirectoryReader.ReadArgsForCall(0)
			Expect(formReadArgs).To(Equal("some-forms-directory"))

			productName, generatedMetadataContents, actualConfig := fakeTileWriter.WriteArgsForCall(0)
			Expect(productName).To(Equal("some-product-name"))
			Expect(generatedMetadataContents).To(MatchYAML(`
icon_image: some-icon-image
name: some-product-name
custom_variable: some-variable-value
variable_from_file: some-variable-value-from-file
releases:
- name: some-release-1
  file: release1.tgz
  version: 1.2.3
- name: some-release-2
  file: release2.tgz
  version: 2.3.4
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
form_types:
- name: some-form
  label: some-form-label
`))
			Expect(actualConfig).To(Equal(builder.WriteInput{
				EmbedPaths:           []string{"some-embed-path"},
				MigrationDirectories: []string{"some-migrations-directory", "some-other-migrations-directory"},
				OutputFile:           "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
				ReleaseDirectories:   []string{otherReleasesDirectory, someReleasesDirectory},
			}))
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

				Expect(fakeMetadataBuilder.BuildCallCount()).To(Equal(1))

				buildInput := fakeMetadataBuilder.BuildArgsForCall(0)
				Expect(buildInput.FormDirectories).To(BeEmpty())
				Expect(buildInput.IconPath).To(Equal("some-icon-path"))
				Expect(buildInput.InstanceGroupDirectories).To(BeEmpty())
				Expect(buildInput.JobDirectories).To(BeEmpty())
				Expect(buildInput.MetadataPath).To(Equal("some-metadata"))
				Expect(buildInput.RuntimeConfigDirectories).To(BeEmpty())
				Expect(buildInput.BOSHVariableDirectories).To(BeEmpty())
				Expect(buildInput.Version).To(Equal("1.2.3"))

				Expect(fakeStemcellManifestReader.ReadCallCount()).To(Equal(0))
			})

			It("calls the tile writer", func() {
				generatedMetadata.Metadata = builder.Metadata{
					"releases": []string{"$(release \"some-release-1\")"},
				}
				fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

				err := bake.Execute([]string{
					"--icon", "some-icon-path",
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
					"--releases-directory", someReleasesDirectory,
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
- name: some-release-1
  file: release1.tgz
  version: 1.2.3
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
`))
				Expect(actualConfig).To(Equal(builder.WriteInput{
					OutputFile:         "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
					ReleaseDirectories: []string{someReleasesDirectory},
				}))
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
				Expect(generatedMetadataContents).To(MatchYAML(`
icon_image: some-icon-image
name: some-product-name
custom_variable: some-variable-value
variable_from_file: override-variable-from-other-file
some_other_variable_from_file: some-other-variable-value-from-file
releases:
- name: some-release-1
  file: release1.tgz
  version: 1.2.3
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
`))
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

			Context("when the release tgz file does not exist but is provided", func() {
				It("returns an error", func() {
					generatedMetadata.Metadata = builder.Metadata{
						"releases": []string{"$(release \"some-release-does-not-exist\")"},
					}
					fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

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
					Expect(err.Error()).To(ContainSubstring("could not find release with name 'some-release-does-not-exist'"))
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

			Context("when the stemcell helper is used without providing the stemcell-tarball flag", func() {
				It("returns an error", func() {
					generatedMetadata.Metadata = map[string]interface{}{"stemcell": `$( stemcell )`}
					fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "unused",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("--stemcell-tarball"))
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

			Context("when the form helper is used without providing the forms-directory flag", func() {
				It("returns an error", func() {
					generatedMetadata.Metadata = map[string]interface{}{"form_types": `$( form "some-form" )`}
					fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "unused",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--version", "1.2.3",
					})

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("--forms-directory"))
				})
			})

			Context("when the requested form name is not found", func() {
				It("returns an error", func() {
					generatedMetadata.Metadata = map[string]interface{}{"form_types": `$( form "invalid-form" )`}
					fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)

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
					Expect(err.Error()).To(ContainSubstring("invalid-form"))
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
			command := commands.NewBake(fakeMetadataBuilder, fakeTileWriter, fakeLogger, fakeReleaseManifestReader, fakeStemcellManifestReader, fakeFormDirectoryReader)
			Expect(command.Usage()).To(Equal(jhandacommands.Usage{
				Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
				ShortDescription: "bakes a tile",
				Flags:            command.Options,
			}))
		})
	})
})
