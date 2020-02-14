package commands_test

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/builder"
	. "github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("Bake", func() {
	var (
		fakeBOSHVariablesService     *fakes.BOSHVariablesService
		fakeFormsService             *fakes.FormsService
		fakeIconService              *fakes.IconService
		fakeInstanceGroupsService    *fakes.InstanceGroupsService
		fakeInterpolator             *fakes.Interpolator
		fakeJobsService              *fakes.JobsService
		fakeLogger                   *log.Logger
		fakeMetadataService          *fakes.MetadataService
		fakePropertiesService        *fakes.PropertiesService
		fakeReleasesService          *fakes.ReleasesService
		fakeRuntimeConfigsService    *fakes.RuntimeConfigsService
		fakeStemcellService          *fakes.StemcellService
		fakeTemplateVariablesService *fakes.TemplateVariablesService
		fakeTileWriter               *fakes.TileWriter
		fakeChecksummer              *fakes.Checksummer

		otherReleasesDirectory string
		someReleasesDirectory  string
		tmpDir                 string
		variableFiles []string
		variables []string

		bake *Bake
		options *BakeOptions
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

		fakeBOSHVariablesService = &fakes.BOSHVariablesService{}
		fakeFormsService = &fakes.FormsService{}
		fakeIconService = &fakes.IconService{}
		fakeInstanceGroupsService = &fakes.InstanceGroupsService{}
		fakeInterpolator = &fakes.Interpolator{}
		fakeJobsService = &fakes.JobsService{}
		fakeLogger = log.New(GinkgoWriter, "", 0)
		fakeMetadataService = &fakes.MetadataService{}
		fakePropertiesService = &fakes.PropertiesService{}
		fakeReleasesService = &fakes.ReleasesService{}
		fakeRuntimeConfigsService = &fakes.RuntimeConfigsService{}
		fakeStemcellService = &fakes.StemcellService{}
		fakeTemplateVariablesService = &fakes.TemplateVariablesService{}
		fakeTileWriter = &fakes.TileWriter{}
		fakeChecksummer = &fakes.Checksummer{}

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

		fakeStemcellService.FromTarballReturns(builder.StemcellManifest{
			Version:         "2.3.4",
			OperatingSystem: "an-operating-system",
		}, nil)

		fakeFormsService.FromDirectoriesReturns(map[string]interface{}{
			"some-form": builder.Metadata{
				"name":  "some-form",
				"label": "some-form-label",
			},
		}, nil)

		fakeBOSHVariablesService.FromDirectoriesReturns(map[string]interface{}{
			"some-secret": builder.Metadata{
				"name": "some-secret",
				"type": "password",
			},
		}, nil)

		fakeInstanceGroupsService.FromDirectoriesReturns(map[string]interface{}{
			"some-instance-group": builder.Metadata{
				"name":     "some-instance-group",
				"manifest": "some-manifest",
				"provides": "some-link",
				"release":  "some-release",
			},
		}, nil)

		fakeJobsService.FromDirectoriesReturns(map[string]interface{}{
			"some-job": builder.Metadata{
				"name":     "some-job",
				"release":  "some-release",
				"consumes": "some-link",
			},
		}, nil)

		fakePropertiesService.FromDirectoriesReturns(map[string]interface{}{
			"some-property": builder.Metadata{
				"name":         "some-property",
				"type":         "boolean",
				"configurable": true,
				"default":      false,
			},
		}, nil)

		fakeRuntimeConfigsService.FromDirectoriesReturns(map[string]interface{}{
			"some-runtime-config": builder.Metadata{
				"name":           "some-runtime-config",
				"runtime_config": "some-addon-runtime-config",
			},
		}, nil)

		fakeIconService.EncodeReturns("some-encoded-icon", nil)

		fakeMetadataService.ReadReturns([]byte("some-metadata"), nil)

		fakeInterpolator.InterpolateReturns([]byte("some-interpolated-metadata"), nil)


		variableFiles = []string{"some-variables-file"}
		variables = []string{"some-variable=some-variable-value"}
		options = &BakeOptions{
			EmbedPaths: []string{"some-embed-path"},
			FormDirectories: []string{"some-forms-directory"},
			IconPath: "some-icon-path",
			InstanceGroupDirectories: []string{"some-instance-groups-directory"},
			JobDirectories: []string{"some-jobs-directory"},
			Metadata: "some-metadata",
			OutputFile: "some-output-dir/some-product-file-1.2.3-build.4",
			PropertyDirectories: []string{"some-properties-directory"},
			ReleaseDirectories: []string{otherReleasesDirectory, someReleasesDirectory},
			RuntimeConfigDirectories: []string{"some-other-runtime-configs-directory", "some-runtime-configs-directory"},
			StemcellTarball: "some-stemcell-tarball",
			BOSHVariableDirectories: []string{ "some-other-variables-directory", "some-variables-directory" },
			Version: "1.2.3",
			MigrationDirectories: []string{ "some-migrations-directory", "some-other-migrations-directory"},
			Sha256: true,
		}
	})

	JustBeforeEach(func() {
		b := NewBake(
			*options,
			"Kilnfile",
			variables,
			variableFiles,
			fakeInterpolator,
			fakeTileWriter,
			fakeLogger,
			fakeTemplateVariablesService,
			fakeBOSHVariablesService,
			fakeReleasesService,
			fakeStemcellService,
			fakeFormsService,
			fakeInstanceGroupsService,
			fakeJobsService,
			fakePropertiesService,
			fakeRuntimeConfigsService,
			fakeIconService,
			fakeMetadataService,
			fakeChecksummer,
		)
		bake = &b
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Run", func() {
		It("builds the tile", func() {
			err := bake.Run(nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTemplateVariablesService.FromPathsAndPairsCallCount()).To(Equal(1))
			varFiles, variables := fakeTemplateVariablesService.FromPathsAndPairsArgsForCall(0)
			Expect(varFiles).To(Equal([]string{"some-variables-file"}))
			Expect(variables).To(Equal([]string{"some-variable=some-variable-value"}))

			Expect(fakeBOSHVariablesService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakeBOSHVariablesService.FromDirectoriesArgsForCall(0)).To(Equal([]string{
				"some-other-variables-directory",
				"some-variables-directory",
			}))

			Expect(fakeReleasesService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakeReleasesService.FromDirectoriesArgsForCall(0)).To(Equal([]string{otherReleasesDirectory, someReleasesDirectory}))

			Expect(fakeStemcellService.FromTarballCallCount()).To(Equal(1))
			Expect(fakeStemcellService.FromTarballArgsForCall(0)).To(Equal("some-stemcell-tarball"))

			Expect(fakeFormsService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakeFormsService.FromDirectoriesArgsForCall(0)).To(Equal([]string{"some-forms-directory"}))

			Expect(fakeInstanceGroupsService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakeInstanceGroupsService.FromDirectoriesArgsForCall(0)).To(Equal([]string{"some-instance-groups-directory"}))

			Expect(fakeJobsService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakeJobsService.FromDirectoriesArgsForCall(0)).To(Equal([]string{"some-jobs-directory"}))

			Expect(fakePropertiesService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakePropertiesService.FromDirectoriesArgsForCall(0)).To(Equal([]string{"some-properties-directory"}))

			Expect(fakeRuntimeConfigsService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakeRuntimeConfigsService.FromDirectoriesArgsForCall(0)).To(Equal([]string{
				"some-other-runtime-configs-directory",
				"some-runtime-configs-directory",
			}))

			Expect(fakeIconService.EncodeCallCount()).To(Equal(1))
			Expect(fakeIconService.EncodeArgsForCall(0)).To(Equal("some-icon-path"))

			Expect(fakeMetadataService.ReadCallCount()).To(Equal(1))
			Expect(fakeMetadataService.ReadArgsForCall(0)).To(Equal("some-metadata"))

			Expect(fakeInterpolator.InterpolateCallCount()).To(Equal(1))

			input, metadata := fakeInterpolator.InterpolateArgsForCall(0)
			Expect(input).To(Equal(builder.InterpolateInput{
				Version: "1.2.3",
				BOSHVariables: map[string]interface{}{
					"some-secret": builder.Metadata{
						"name": "some-secret",
						"type": "password",
					},
				},
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
				IconImage: "some-encoded-icon",
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

			Expect(string(metadata)).To(Equal("some-metadata"))

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

			Expect(fakeChecksummer.SumCallCount()).To(Equal(1))
			outputFilePath := fakeChecksummer.SumArgsForCall(0)
			Expect(outputFilePath).To(Equal(filepath.Join("some-output-dir", "some-product-file-1.2.3-build.4")))
		})

		Context("when the --sha256 flag is not specified", func() {
			BeforeEach(func() {
				options.Sha256 = false
			})

			It("does not calculate a checksum", func() {
				err := bake.Run(nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeChecksummer.SumCallCount()).To(Equal(0))
			})
		})

		Context("when the optional flags are not specified", func() {
			BeforeEach(func() {
				variables = nil
				variableFiles = nil
				options = &BakeOptions{
					Metadata: "some-metadata",
					ReleaseDirectories: []string{someReleasesDirectory},
					OutputFile: "some-output-dir/some-product-file-1.2.3-build.4",
					Version: "1.2.3",
				}
			})

			It("builds the metadata", func() {
				err := bake.Run(nil)

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

				variableFileData := map[string]string{
					"some-variable-from-file":       "override-variable-from-other-file",
					"some-other-variable-from-file": "some-other-variable-value-from-file",
				}
				variableFileContents, err := yaml.Marshal(&variableFileData)
				Expect(err).NotTo(HaveOccurred())

				n, err := otherVariableFile.Write(variableFileContents)
				Expect(err).NotTo(HaveOccurred())
				Expect(variableFileContents).To(HaveLen(n))

				variableFiles = []string{"some-variable-file-1","some-variable-file-2"}
			})

			It("interpolates variables from both files", func() {
				err := bake.Run(nil)
				Expect(err).NotTo(HaveOccurred())

				generatedMetadataContents, _ := fakeTileWriter.WriteArgsForCall(0)
				Expect(generatedMetadataContents).To(HelpfullyMatchYAML("some-interpolated-metadata"))
			})
		})

		Context("when stemcells-directory flag is specified", func() {
			BeforeEach(func() {
				options.StemcellsDirectories = []string{"some-stemcells-directory", "some-other-stemcells-directory"}
				options.StemcellTarball = ""
			})

			It("correcty parses stemcell directory arguments", func() {
				err := bake.Run(nil)

				Expect(err).NotTo(HaveOccurred())

				Expect(fakeStemcellService.FromDirectoriesCallCount()).To(Equal(1))
				Expect(fakeStemcellService.FromDirectoriesArgsForCall(0)).To(Equal([]string{
					"some-stemcells-directory",
					"some-other-stemcells-directory",
				}))
			})
		})

		Context("when neither stemcell tarball nor stemcell directories are specified", func() {
			BeforeEach(func() {
				options.StemcellTarball = ""
				options.StemcellsDirectories = nil
			})

			It("renders the stemcell criteria in tile metadata from that specified the Kilnfile.lock", func() {
				err := bake.Run(nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeStemcellService.FromKilnfileCallCount()).To(Equal(1))
				Expect(fakeStemcellService.FromKilnfileArgsForCall(0)).To(Equal("Kilnfile"))
			})
		})

		Context("failure cases", func() {
			Context("when the template variables service errors", func() {
				BeforeEach(func() {
					fakeTemplateVariablesService.FromPathsAndPairsReturns(nil, errors.New("parsing template variables failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to parse template variables: parsing template variables failed"))
				})
			})

			Context("when the icon service fails", func() {
				BeforeEach(func() {
					fakeIconService.EncodeReturns("", errors.New("encoding icon failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to encode icon: encoding icon failed"))
				})
			})

			Context("when the metadata service fails", func() {
				BeforeEach(func() {
					fakeMetadataService.ReadReturns(nil, errors.New("reading metadata failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to read metadata: reading metadata failed"))
				})
			})

			Context("when the releases service fails", func() {
				BeforeEach(func() {
					fakeReleasesService.FromDirectoriesReturns(nil, errors.New("parsing releases failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to parse releases: parsing releases failed"))
				})
			})

			Context("when the stemcell service fails", func() {
				BeforeEach(func() {
					fakeStemcellService.FromTarballReturns(nil, errors.New("parsing stemcell failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to parse stemcell: parsing stemcell failed"))
				})
			})

			Context("when the forms service fails", func() {
				BeforeEach(func() {
					fakeFormsService.FromDirectoriesReturns(nil, errors.New("parsing forms failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to parse forms: parsing forms failed"))
				})
			})

			Context("when the instance groups service fails", func() {
				BeforeEach(func() {
					fakeInstanceGroupsService.FromDirectoriesReturns(nil, errors.New("parsing instance groups failed"))
				})
				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to parse instance groups: parsing instance groups failed"))
				})
			})

			Context("when the jobs service fails", func() {
				BeforeEach(func() {
					fakeJobsService.FromDirectoriesReturns(nil, errors.New("parsing jobs failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to parse jobs: parsing jobs failed"))
				})
			})

			Context("when the properties service fails", func() {
				BeforeEach(func() {
					fakePropertiesService.FromDirectoriesReturns(nil, errors.New("parsing properties failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to parse properties: parsing properties failed"))
				})
			})

			Context("when the runtime configs service fails", func() {
				BeforeEach(func() {
					fakeRuntimeConfigsService.FromDirectoriesReturns(nil, errors.New("parsing runtime configs failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("failed to parse runtime configs: parsing runtime configs failed"))
				})
			})

			Context("when the template interpolator returns an error", func() {
				BeforeEach(func() {
					fakeInterpolator.InterpolateReturns(nil, errors.New("some-error"))
				})

				It("returns the error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError(ContainSubstring("some-error")))
				})
			})

			Context("when the release-tarball flag is missing and we are stubbing releases", func() {
				BeforeEach(func() {
					options.ReleaseDirectories = nil
					options.StubReleases = true
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the output-file flag is missing", func() {
				BeforeEach(func() {
					options.OutputFile = ""
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("--output-file must be provided unless using --metadata-only"))
				})
			})

			//todo: When --stemcell-tarball is remove, delete this test
			Context("when both the --stemcell-tarball and --stemcells-directory are provided", func() {
				BeforeEach(func() {
					options.StemcellTarball = "some-stemcell-tarball"
					options.StemcellsDirectories = []string{"some-stemcell-directory"}
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("--stemcell-tarball cannot be provided when using --stemcells-directory"))
				})
			})

			Context("when both the output-file and metadata-only flags are provided", func() {
				BeforeEach(func() {
					options.OutputFile = "some-output-dir/some-product-file-1.2.3-build.4"
					options.MetadataOnly =  true
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("--output-file cannot be provided when using --metadata-only"))
				})
			})

			Context("when the jobs-directory flag is passed without the instance-groups-directory flag", func() {
				BeforeEach(func() {
					options.JobDirectories = []string{"some-jobs-directory"}
					options.InstanceGroupDirectories = nil
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError("--jobs-directory flag requires --instance-groups-directory to also be specified"))
				})
			})

			Context("when the checksummer returns an error", func() {
				BeforeEach(func() {
					fakeChecksummer.SumReturns(errors.New("failed"))
				})

				It("returns an error", func() {
					err := bake.Run(nil)
					Expect(err).To(MatchError(ContainSubstring("failed to calculate checksum: failed")))
				})
			})
		})
	})
})
