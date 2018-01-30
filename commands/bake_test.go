package commands_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
)

var _ = Describe("bake", func() {
	var (
		fakeMetadataBuilder               *fakes.MetadataBuilder
		fakeStemcellManifestReader        *fakes.PartReader
		fakeFormDirectoryReader           *fakes.DirectoryReader
		fakeInstanceGroupDirectoryReader  *fakes.DirectoryReader
		fakeJobsDirectoryReader           *fakes.DirectoryReader
		fakePropertyDirectoryReader       *fakes.DirectoryReader
		fakeRuntimeConfigsDirectoryReader *fakes.DirectoryReader
		fakeInterpolator                  *fakes.Interpolator
		fakeTileWriter                    *fakes.TileWriter
		fakeLogger                        *fakes.Logger
		fakeTemplateVariablesService      *fakes.TemplateVariablesService
		fakeReleasesService               *fakes.ReleasesService

		generatedMetadata      builder.GeneratedMetadata
		otherReleasesDirectory string
		someReleasesDirectory  string
		tmpDir                 string

		bake commands.Bake
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "command-test")
		Expect(err).NotTo(HaveOccurred())

		someReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		nonTarballRelease := filepath.Join(someReleasesDirectory, "some-broken-release")
		err = ioutil.WriteFile(nonTarballRelease, []byte(""), 0644)
		Expect(err).NotTo(HaveOccurred())

		fakeMetadataBuilder = &fakes.MetadataBuilder{}
		fakeStemcellManifestReader = &fakes.PartReader{}
		fakeFormDirectoryReader = &fakes.DirectoryReader{}
		fakeInstanceGroupDirectoryReader = &fakes.DirectoryReader{}
		fakeJobsDirectoryReader = &fakes.DirectoryReader{}
		fakePropertyDirectoryReader = &fakes.DirectoryReader{}
		fakeRuntimeConfigsDirectoryReader = &fakes.DirectoryReader{}
		fakeInterpolator = &fakes.Interpolator{}
		fakeTileWriter = &fakes.TileWriter{}
		fakeLogger = &fakes.Logger{}
		fakeTemplateVariablesService = &fakes.TemplateVariablesService{}
		fakeReleasesService = &fakes.ReleasesService{}

		fakeTemplateVariablesService.FromPathsAndPairsReturns(map[string]interface{}{
			"some-variable-from-file": "some-variable-value-from-file",
			"some-variable":           "some-variable-value",
		}, nil)

		fakeReleasesService.FromDirectoriesReturns(map[string]interface{}{
			"some-release-1": builder.ReleaseManifest{
				Name:    "some-release-1",
				Version: "1.2.3",
				File:    "release1.tgz",
			},
			"some-release-2": builder.ReleaseManifest{
				Name:    "some-release-2",
				Version: "2.3.4",
				File:    "release2.tar.gz",
			},
		}, nil)

		fakeStemcellManifestReader.ReadReturns(builder.Part{
			Metadata: builder.StemcellManifest{
				Version:         "2.3.4",
				OperatingSystem: "an-operating-system",
			},
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

		fakeInstanceGroupDirectoryReader.ReadReturns([]builder.Part{
			{
				Name: "some-instance-group",
				Metadata: builder.Metadata{
					"name":     "some-instance-group",
					"manifest": "some-manifest",
					"provides": "some-link",
					"release":  "some-release",
				},
			},
		}, nil)

		fakeJobsDirectoryReader.ReadReturns([]builder.Part{
			{
				Name: "some-job",
				Metadata: builder.Metadata{
					"name":     "some-job",
					"release":  "some-release",
					"consumes": "some-link",
				},
			},
		}, nil)

		fakePropertyDirectoryReader.ReadReturns([]builder.Part{
			{
				Name: "some-property",
				Metadata: builder.Metadata{
					"name":         "some-property",
					"type":         "boolean",
					"configurable": true,
					"default":      false,
				},
			},
		}, nil)

		fakeRuntimeConfigsDirectoryReader.ReadReturns([]builder.Part{
			{
				Name: "some-runtime-config",
				Metadata: builder.Metadata{
					"name":           "some-runtime-config",
					"runtime_config": "some-addon-runtime-config",
				},
			},
		}, nil)

		generatedMetadata = builder.GeneratedMetadata{IconImage: "some-icon-image"}
		fakeMetadataBuilder.BuildReturns(generatedMetadata, nil)
		fakeInterpolator.InterpolateReturns([]byte("some-interpolated-metadata"), nil)

		bake = commands.NewBake(
			fakeMetadataBuilder,
			fakeInterpolator,
			fakeTileWriter,
			fakeLogger,
			fakeStemcellManifestReader,
			fakeFormDirectoryReader,
			fakeInstanceGroupDirectoryReader,
			fakeJobsDirectoryReader,
			fakePropertyDirectoryReader,
			fakeRuntimeConfigsDirectoryReader,
			func(interface{}) ([]byte, error) { return []byte("some-yaml"), nil },
			fakeTemplateVariablesService,
			fakeReleasesService,
		)
	})

	AfterEach(func() {
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
				"--variables-file", "some-variables-file",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTemplateVariablesService.FromPathsAndPairsCallCount()).To(Equal(1))
			varFiles, variables := fakeTemplateVariablesService.FromPathsAndPairsArgsForCall(0)
			Expect(varFiles).To(Equal([]string{"some-variables-file"}))
			Expect(variables).To(Equal([]string{"some-variable=some-variable-value"}))

			Expect(fakeReleasesService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakeReleasesService.FromDirectoriesArgsForCall(0)).To(Equal([]string{otherReleasesDirectory, someReleasesDirectory}))

			Expect(fakeStemcellManifestReader.ReadCallCount()).To(Equal(1))
			Expect(fakeStemcellManifestReader.ReadArgsForCall(0)).To(Equal("some-stemcell-tarball"))

			Expect(fakeFormDirectoryReader.ReadCallCount()).To(Equal(1))
			Expect(fakeFormDirectoryReader.ReadArgsForCall(0)).To(Equal("some-forms-directory"))

			Expect(fakeInstanceGroupDirectoryReader.ReadCallCount()).To(Equal(1))
			Expect(fakeInstanceGroupDirectoryReader.ReadArgsForCall(0)).To(Equal("some-instance-groups-directory"))

			Expect(fakeJobsDirectoryReader.ReadCallCount()).To(Equal(1))
			Expect(fakeJobsDirectoryReader.ReadArgsForCall(0)).To(Equal("some-jobs-directory"))

			Expect(fakePropertyDirectoryReader.ReadCallCount()).To(Equal(1))
			Expect(fakePropertyDirectoryReader.ReadArgsForCall(0)).To(Equal("some-properties-directory"))

			Expect(fakeRuntimeConfigsDirectoryReader.ReadCallCount()).To(Equal(2))
			Expect(fakeRuntimeConfigsDirectoryReader.ReadArgsForCall(0)).To(Equal("some-other-runtime-configs-directory"))
			Expect(fakeRuntimeConfigsDirectoryReader.ReadArgsForCall(1)).To(Equal("some-runtime-configs-directory"))

			Expect(fakeMetadataBuilder.BuildCallCount()).To(Equal(1))
			expectedBuildInput := builder.BuildInput{
				IconPath:                "some-icon-path",
				MetadataPath:            "some-metadata",
				BOSHVariableDirectories: []string{"some-other-variables-directory", "some-variables-directory"},
			}
			Expect(fakeMetadataBuilder.BuildArgsForCall(0)).To(Equal(expectedBuildInput))

			Expect(fakeInterpolator.InterpolateCallCount()).To(Equal(1))

			input, metadata := fakeInterpolator.InterpolateArgsForCall(0)
			Expect(input).To(Equal(builder.InterpolateInput{
				Version: "1.2.3",
				Variables: map[string]interface{}{
					"some-variable-from-file": "some-variable-value-from-file",
					"some-variable":           "some-variable-value",
				},
				ReleaseManifests: map[string]interface{}{
					"some-release-1": builder.ReleaseManifest{
						Name:    "some-release-1",
						Version: "1.2.3",
						File:    "release1.tgz",
					},
					"some-release-2": builder.ReleaseManifest{
						Name:    "some-release-2",
						Version: "2.3.4",
						File:    "release2.tar.gz",
					},
				},
				StemcellManifest: builder.StemcellManifest{
					Version:         "2.3.4",
					OperatingSystem: "an-operating-system",
				},
				FormTypes: map[string]interface{}{
					"some-form": builder.Metadata{
						"name":  "some-form",
						"label": "some-form-label",
					},
				},
				IconImage: "some-icon-image",
				InstanceGroups: map[string]interface{}{
					"some-instance-group": builder.Metadata{
						"name":     "some-instance-group",
						"manifest": "some-manifest",
						"provides": "some-link",
						"release":  "some-release",
					},
				},
				Jobs: map[string]interface{}{
					"some-job": builder.Metadata{
						"name":     "some-job",
						"release":  "some-release",
						"consumes": "some-link",
					},
				},
				PropertyBlueprints: map[string]interface{}{
					"some-property": builder.Metadata{
						"name":         "some-property",
						"type":         "boolean",
						"configurable": true,
						"default":      false,
					},
				},
				RuntimeConfigs: map[string]interface{}{
					"some-runtime-config": builder.Metadata{
						"name":           "some-runtime-config",
						"runtime_config": "some-addon-runtime-config",
					},
				},
			}))

			Expect(string(metadata)).To(Equal("some-yaml"))

			Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))
			metadata, writeInput := fakeTileWriter.WriteArgsForCall(0)
			Expect(string(metadata)).To(Equal("some-interpolated-metadata"))
			Expect(writeInput).To(Equal(builder.WriteInput{
				OutputFile:           filepath.Join("some-output-dir", "some-product-file-1.2.3-build.4"),
				StubReleases:         false,
				MigrationDirectories: []string{"some-migrations-directory", "some-other-migrations-directory"},
				ReleaseDirectories:   []string{otherReleasesDirectory, someReleasesDirectory},
				EmbedPaths:           []string{"some-embed-path"},
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
					"--variables-file", "some-variable-file-1",
					"--variables-file", "some-variable-file-2",
					"--version", "1.2.3",
				})

				Expect(err).NotTo(HaveOccurred())

				generatedMetadataContents, _ := fakeTileWriter.WriteArgsForCall(0)
				Expect(generatedMetadataContents).To(MatchYAML("some-interpolated-metadata"))
			})
		})

		Context("failure cases", func() {
			Context("when the template variables service errors", func() {
				It("returns an error", func() {
					fakeTemplateVariablesService.FromPathsAndPairsReturns(nil, errors.New("parsing template variables failed"))

					err := bake.Execute([]string{
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--icon", "some-icon-path",
						"--releases-directory", someReleasesDirectory,
					})
					Expect(err).To(MatchError("failed to parse template variables: parsing template variables failed"))
				})
			})

			Context("when the releases service fails", func() {
				It("returns an error", func() {
					fakeReleasesService.FromDirectoriesReturns(nil, errors.New("parsing releases failed"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--releases-directory", someReleasesDirectory,
					})

					Expect(err).To(MatchError("failed to parse releases: parsing releases failed"))
				})
			})

			Context("when the stemcell manifest reader returns an error", func() {
				It("returns an error", func() {
					fakeStemcellManifestReader.ReadReturns(builder.Part{}, errors.New("some-error"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError(ContainSubstring("some-error")))
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

					Expect(err).To(MatchError(ContainSubstring("some-error")))
				})
			})

			Context("when the instance group directory reader returns an error", func() {
				It("returns an error", func() {
					fakeInstanceGroupDirectoryReader.ReadReturns(nil, errors.New("some-error"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--forms-directory", "some-form-directory",
						"--instance-groups-directory", "some-instance-group-directory",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError(ContainSubstring("some-error")))
				})
			})

			Context("when the property directory reader returns an error", func() {
				It("returns an error", func() {
					fakePropertyDirectoryReader.ReadReturns(nil, errors.New("some-error"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--forms-directory", "some-form-directory",
						"--instance-groups-directory", "some-instance-group-directory",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError(ContainSubstring("some-error")))
				})
			})

			Context("when the runtime config directory reader returns an error", func() {
				It("returns an error", func() {
					fakeRuntimeConfigsDirectoryReader.ReadReturns(nil, errors.New("some-error"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--runtime-configs-directory", "some-runtime-configs-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--forms-directory", "some-form-directory",
						"--instance-groups-directory", "some-instance-group-directory",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError(ContainSubstring("some-error")))
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

					Expect(err).To(MatchError(ContainSubstring("some-error")))
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

					Expect(err).To(MatchError("missing required flag \"--icon\""))
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

					Expect(err).To(MatchError("missing required flag \"--metadata\""))
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

					Expect(err).To(MatchError("missing required flag \"--releases-directory\""))
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

					Expect(err).To(MatchError("missing required flag \"--output-file\""))
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

					Expect(err).To(MatchError(ContainSubstring("non-existant-flag")))
				})
			})
		})
	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			Expect(bake.Usage()).To(Equal(jhanda.Usage{
				Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
				ShortDescription: "bakes a tile",
				Flags:            bake.Options,
			}))
		})
	})
})
