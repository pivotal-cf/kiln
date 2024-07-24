package commands_test

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pivotal-cf/kiln/pkg/cargo"

	"github.com/pivotal-cf/kiln/pkg/bake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("Bake", func() {
	var (
		fakeInterpolator *fakes.Interpolator
		fakeLogger       *log.Logger

		fakeIconService     *fakes.IconService
		fakeStemcellService *fakes.StemcellService
		fakeReleasesService *fakes.FromDirectories
		fakeFetcher         *fakes.Fetch

		fakeTemplateVariablesService *fakes.TemplateVariablesService
		fakeMetadataService          *fakes.MetadataService

		fakeBOSHVariablesService,
		fakeFormsService,
		fakeInstanceGroupsService,
		fakeJobsService,
		fakePropertiesService,
		fakeRuntimeConfigsService *fakes.MetadataTemplatesParser

		fakeTileWriter  *fakes.TileWriter
		fakeChecksummer *fakes.Checksummer

		fakeFilesystem  *fakes.FileSystem
		fakeHomeDirFunc func() (string, error)

		otherReleasesDirectory string
		someReleasesDirectory  string
		tmpDir                 string

		fakeBakeRecordFunc *fakeWriteBakeRecordFunc

		bake commands.Bake
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "command-test")
		Expect(err).NotTo(HaveOccurred())

		someReleasesDirectory, err = os.MkdirTemp(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = os.MkdirTemp(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		nonTarballRelease := filepath.Join(someReleasesDirectory, "some-broken-release")
		err = os.WriteFile(nonTarballRelease, []byte(""), 0o644)
		Expect(err).NotTo(HaveOccurred())

		fakeTileWriter = &fakes.TileWriter{}
		fakeChecksummer = &fakes.Checksummer{}
		fakeIconService = &fakes.IconService{}
		fakeInterpolator = &fakes.Interpolator{}
		fakeBakeRecordFunc = &fakeWriteBakeRecordFunc{}

		fakeLogger = log.New(GinkgoWriter, "", 0)

		fakeStemcellService = &fakes.StemcellService{}
		fakeReleasesService = &fakes.FromDirectories{}

		fakeTemplateVariablesService = &fakes.TemplateVariablesService{}
		fakeMetadataService = &fakes.MetadataService{}
		fakeInstanceGroupsService = &fakes.MetadataTemplatesParser{}
		fakeBOSHVariablesService = &fakes.MetadataTemplatesParser{}
		fakeFormsService = &fakes.MetadataTemplatesParser{}
		fakeJobsService = &fakes.MetadataTemplatesParser{}
		fakePropertiesService = &fakes.MetadataTemplatesParser{}
		fakeRuntimeConfigsService = &fakes.MetadataTemplatesParser{}
		fakeFilesystem = &fakes.FileSystem{}
		fakeVersionInfo := &fakes.FileInfo{}
		fileVersion := "some-version"
		fakeVersionInfo.SizeReturns(int64(len(fileVersion)))
		fakeVersionInfo.NameReturns("version")
		fakeFilesystem.StatReturns(fakeVersionInfo, nil)
		result1 := &fakes.File{}
		result1.ReadReturns(0, nil)
		fakeFilesystem.OpenReturns(result1, nil)
		fakeHomeDirFunc = func() (string, error) {
			return "/home/", nil
		}

		fakeTemplateVariablesService.FromPathsAndPairsReturns(map[string]any{
			"some-variable-from-file": "some-variable-value-from-file",
			"some-variable":           "some-variable-value",
		}, nil)

		fakeReleasesService.FromDirectoriesReturns(map[string]any{
			"some-release-1": proofing.Release{
				Name:    "some-release-1",
				Version: "1.2.3",
				File:    "release1.tgz",
			},
			"some-release-2": proofing.Release{
				Name:    "some-release-2",
				Version: "2.3.4",
				File:    "release2.tar.gz",
			},
		}, nil)

		fakeStemcellService.FromTarballReturns(builder.StemcellManifest{
			Version:         "2.3.4",
			OperatingSystem: "an-operating-system",
		}, nil)

		fakeFormsService.ParseMetadataTemplatesReturns(map[string]any{
			"some-form": builder.Metadata{
				"name":  "some-form",
				"label": "some-form-label",
			},
		}, nil)

		fakeBOSHVariablesService.ParseMetadataTemplatesReturns(map[string]any{
			"some-secret": builder.Metadata{
				"name": "some-secret",
				"type": "password",
			},
		}, nil)

		fakeInstanceGroupsService.ParseMetadataTemplatesReturns(map[string]any{
			"some-instance-group": builder.Metadata{
				"name":     "some-instance-group",
				"manifest": "some-manifest",
				"provides": "some-link",
				"release":  "some-release",
			},
		}, nil)

		fakeJobsService.ParseMetadataTemplatesReturns(map[string]any{
			"some-job": builder.Metadata{
				"name":     "some-job",
				"release":  "some-release",
				"consumes": "some-link",
			},
		}, nil)

		fakePropertiesService.ParseMetadataTemplatesReturns(map[string]any{
			"some-property": builder.Metadata{
				"name":         "some-property",
				"type":         "boolean",
				"configurable": true,
				"default":      false,
			},
		}, nil)

		fakeRuntimeConfigsService.ParseMetadataTemplatesReturns(map[string]any{
			"some-runtime-config": builder.Metadata{
				"name":           "some-runtime-config",
				"runtime_config": "some-addon-runtime-config",
			},
		}, nil)

		fakeIconService.EncodeReturns("some-encoded-icon", nil)

		fakeMetadataService.ReadReturns([]byte("some-metadata"), nil)

		fakeInterpolator.InterpolateReturns([]byte("some-interpolated-metadata"), nil)

		fakeFetcher = &fakes.Fetch{}
		fakeFetcher.ExecuteReturns(nil)
		bake = commands.NewBakeWithInterfaces(fakeInterpolator, fakeTileWriter, fakeLogger, fakeLogger, fakeTemplateVariablesService, fakeBOSHVariablesService, fakeReleasesService, fakeStemcellService, fakeFormsService, fakeInstanceGroupsService, fakeJobsService, fakePropertiesService, fakeRuntimeConfigsService, fakeIconService, fakeMetadataService, fakeChecksummer, fakeFetcher, fakeFilesystem, fakeHomeDirFunc, fakeBakeRecordFunc.call)
		bake = bake.WithKilnfileFunc(func(s string) (cargo.Kilnfile, error) { return cargo.Kilnfile{}, nil })
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Execute", func() {
		It("builds the tile", func() {
			err := bake.Execute([]string{
				"--final",
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
				"--version", "1.2.3", "--migrations-directory", "some-migrations-directory",
				"--migrations-directory", "some-other-migrations-directory",
				"--variable", "some-variable=some-variable-value",
				"--variables-file", "some-variables-file",
				"--sha256",
				"--download-threads", "5",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTemplateVariablesService.FromPathsAndPairsCallCount()).To(Equal(1))
			varFiles, variables := fakeTemplateVariablesService.FromPathsAndPairsArgsForCall(0)
			Expect(len(varFiles)).To(Equal(2))
			Expect(varFiles[0]).To(Equal("some-variables-file"))
			Expect(varFiles[1]).To(Equal("/home/.kiln/credentials.yml"))
			Expect(variables).To(Equal([]string{"some-variable=some-variable-value"}))

			Expect(fakeBOSHVariablesService.ParseMetadataTemplatesCallCount()).To(Equal(1))
			paths, _ := fakeBOSHVariablesService.ParseMetadataTemplatesArgsForCall(0)
			Expect(paths).To(Equal([]string{
				"some-other-variables-directory",
				"some-variables-directory",
			}))

			Expect(fakeReleasesService.FromDirectoriesCallCount()).To(Equal(1))
			Expect(fakeReleasesService.FromDirectoriesArgsForCall(0)).To(Equal([]string{otherReleasesDirectory, someReleasesDirectory}))

			Expect(fakeStemcellService.FromTarballCallCount()).To(Equal(1))
			Expect(fakeStemcellService.FromTarballArgsForCall(0)).To(Equal("some-stemcell-tarball"))

			Expect(fakeFormsService.ParseMetadataTemplatesCallCount()).To(Equal(1))
			paths, _ = fakeFormsService.ParseMetadataTemplatesArgsForCall(0)
			Expect(paths).To(Equal([]string{"some-forms-directory"}))

			Expect(fakeInstanceGroupsService.ParseMetadataTemplatesCallCount()).To(Equal(1))
			paths, _ = fakeInstanceGroupsService.ParseMetadataTemplatesArgsForCall(0)
			Expect(paths).To(Equal([]string{"some-instance-groups-directory"}))

			Expect(fakeJobsService.ParseMetadataTemplatesCallCount()).To(Equal(1))
			paths, _ = fakeJobsService.ParseMetadataTemplatesArgsForCall(0)
			Expect(paths).To(Equal([]string{"some-jobs-directory"}))

			Expect(fakePropertiesService.ParseMetadataTemplatesCallCount()).To(Equal(1))
			paths, _ = fakePropertiesService.ParseMetadataTemplatesArgsForCall(0)
			Expect(paths).To(Equal([]string{"some-properties-directory"}))

			Expect(fakeRuntimeConfigsService.ParseMetadataTemplatesCallCount()).To(Equal(1))
			paths, _ = fakeRuntimeConfigsService.ParseMetadataTemplatesArgsForCall(0)
			Expect(paths).To(Equal([]string{
				"some-other-runtime-configs-directory",
				"some-runtime-configs-directory",
			}))

			Expect(fakeIconService.EncodeCallCount()).To(Equal(1))
			Expect(fakeIconService.EncodeArgsForCall(0)).To(Equal("some-icon-path"))

			Expect(fakeMetadataService.ReadCallCount()).To(Equal(1))
			Expect(fakeMetadataService.ReadArgsForCall(0)).To(Equal("some-metadata"))

			Expect(fakeInterpolator.InterpolateCallCount()).To(Equal(1))

			metadataGitSHA, err := builder.GitMetadataSHA(".", true)
			Expect(err).NotTo(HaveOccurred())

			input, interpolateName, metadata := fakeInterpolator.InterpolateArgsForCall(0)
			Expect(input.MetadataGitSHA).NotTo(BeEmpty())
			Expect(input).To(Equal(builder.InterpolateInput{
				MetadataGitSHA: metadataGitSHA,
				Version:        "1.2.3",
				BOSHVariables: map[string]any{
					"some-secret": builder.Metadata{
						"name": "some-secret",
						"type": "password",
					},
				},
				Variables: map[string]any{
					"some-variable-from-file": "some-variable-value-from-file",
					"some-variable":           "some-variable-value",
				},
				ReleaseManifests: map[string]any{
					"some-release-1": proofing.Release{
						Name:    "some-release-1",
						Version: "1.2.3",
						File:    "release1.tgz",
					},
					"some-release-2": proofing.Release{
						Name:    "some-release-2",
						Version: "2.3.4",
						File:    "release2.tar.gz",
					},
				},
				StemcellManifest: builder.StemcellManifest{
					Version:         "2.3.4",
					OperatingSystem: "an-operating-system",
				},
				FormTypes: map[string]any{
					"some-form": builder.Metadata{
						"name":  "some-form",
						"label": "some-form-label",
					},
				},
				IconImage: "some-encoded-icon",
				InstanceGroups: map[string]any{
					"some-instance-group": builder.Metadata{
						"name":     "some-instance-group",
						"manifest": "some-manifest",
						"provides": "some-link",
						"release":  "some-release",
					},
				},
				Jobs: map[string]any{
					"some-job": builder.Metadata{
						"name":     "some-job",
						"release":  "some-release",
						"consumes": "some-link",
					},
				},
				PropertyBlueprints: map[string]any{
					"some-property": builder.Metadata{
						"name":         "some-property",
						"type":         "boolean",
						"configurable": true,
						"default":      false,
					},
				},
				RuntimeConfigs: map[string]any{
					"some-runtime-config": builder.Metadata{
						"name":           "some-runtime-config",
						"runtime_config": "some-addon-runtime-config",
					},
				},
			}))
			Expect(interpolateName).To(Equal("some-metadata"))
			Expect(string(metadata)).To(Equal("some-metadata"))

			Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))
			metadata, writeInput := fakeTileWriter.WriteArgsForCall(0)
			Expect(string(metadata)).To(Equal("some-interpolated-metadata"))

			Expect(writeInput.ModTime).NotTo(BeZero())
			writeInput.ModTime = time.Time{}

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

			Expect(fakeFetcher.ExecuteCallCount()).To(Equal(2))
			executeArgsForFetch := fakeFetcher.ExecuteArgsForCall(0)
			Expect(executeArgsForFetch).To(Equal([]string{
				"--kilnfile", "",
				"--variables-file", "some-variables-file",
				"--variables-file",
				"/home/.kiln/credentials.yml",
				"--variable", "some-variable=some-variable-value",
				"--download-threads", "5",
				"--no-confirm",
				"--releases-directory",
				otherReleasesDirectory,
			}))
			executeArgsForFetch = fakeFetcher.ExecuteArgsForCall(1)
			Expect(executeArgsForFetch).To(Equal([]string{
				"--kilnfile", "",
				"--variables-file", "some-variables-file",
				"--variables-file",
				"/home/.kiln/credentials.yml",
				"--variable", "some-variable=some-variable-value",
				"--download-threads", "5",
				"--no-confirm",
				"--releases-directory",
				someReleasesDirectory,
			}))

			Expect(fakeBakeRecordFunc.recordPath).To(Equal("some-metadata"), "it informs the bake recorder the path to the metadata template")
			Expect(string(fakeBakeRecordFunc.productTemplate)).To(Equal("some-interpolated-metadata"), "it gives the bake recorder the product template")
		})

		FContext("when the output flag is not set", func() {
			When("the tile-name flag is not provided", func() {
				It("uses the tile as the filename prefix", func() {
					err := bake.Execute([]string{})
					Expect(err).To(Not(HaveOccurred()))
					Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))
					_, input := fakeTileWriter.WriteArgsForCall(0)
					Expect(input.OutputFile).To(Equal(filepath.Join("tile-v1.2.3.pivotal")))
				})
			})
		})

		Context("when bake configuration is in the Kilnfile", func() {
			BeforeEach(func() {
				bake = bake.WithKilnfileFunc(func(s string) (cargo.Kilnfile, error) {
					return cargo.Kilnfile{
						BakeConfigurations: []cargo.BakeConfiguration{
							{TileName: "p-each", Metadata: "peach.yml"},
						},
					}, nil
				})
			})
			When("a metadata flag is not passed", func() {
				It("it uses the value from the bake configuration", func() {
					err := bake.Execute([]string{})
					Expect(err).To(Not(HaveOccurred()))
					Expect(fakeMetadataService.ReadArgsForCall(0)).To(Equal("peach.yml"))
				})
			})
			When("generating metadata", func() {
				It("it uses the value from the bake configuration", func() {
					err := bake.Execute([]string{})
					Expect(err).To(Not(HaveOccurred()))
					input, _, _ := fakeInterpolator.InterpolateArgsForCall(0)
					Expect(input.Variables).To(HaveKeyWithValue("tile_name", "p-each"))
				})
			})
		})
		Context("when bake configuration has multiple options", func() {
			BeforeEach(func() {
				bake = bake.WithKilnfileFunc(func(s string) (cargo.Kilnfile, error) {
					return cargo.Kilnfile{
						BakeConfigurations: []cargo.BakeConfiguration{
							{
								TileName: "p-each",
								Metadata: "peach.yml",
							},
							{
								TileName: "p-air",
								Metadata: "pair.yml",
							},
							{
								TileName: "p-lum",
								Metadata: "plum.yml",
							},
						},
					}, nil
				})
			})
			When("a the tile flag is passed", func() {
				It("it uses the value from the bake configuration with the correct name", func() {
					err := bake.Execute([]string{
						"bake",
						"--tile-name=p-each",
					})
					Expect(err).To(Not(HaveOccurred()))
					Expect(fakeMetadataService.ReadArgsForCall(0)).To(Equal("peach.yml"))
				})
			})
		})
		Context("when --stub-releases is specified", func() {
			It("doesn't fetch releases", func() {
				err := bake.Execute([]string{
					"--embed", "some-embed-path",
					"--forms-directory", "some-forms-directory",
					"--icon", "some-icon-path",
					"--instance-groups-directory", "some-instance-groups-directory",
					"--jobs-directory", "some-jobs-directory",
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
					"--properties-directory", "some-properties-directory",
					"--releases-directory", someReleasesDirectory,
					"--runtime-configs-directory", "some-runtime-configs-directory",
					"--stub-releases", "true",
				})
				Expect(err).To(Not(HaveOccurred()))
				Expect(fakeFetcher.ExecuteCallCount()).To(Equal(0))
			})
		})
		Context("when the --sha256 flag is not specified", func() {
			It("does not calculate a checksum", func() {
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
					"--version", "1.2.3", "--migrations-directory", "some-migrations-directory",
					"--migrations-directory", "some-other-migrations-directory",
					"--variable", "some-variable=some-variable-value",
					"--variables-file", "some-variables-file",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeChecksummer.SumCallCount()).To(Equal(0))
			})
		})

		Context("when the optional flags are not specified", func() {
			It("builds the metadata", func() {
				err := bake.Execute([]string{
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
				otherVariableFile, err = os.CreateTemp(tmpDir, "variables-file")
				Expect(err).NotTo(HaveOccurred())
				defer closeAndIgnoreError(otherVariableFile)

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
				Expect(generatedMetadataContents).To(HelpfullyMatchYAML("some-interpolated-metadata"))
			})
		})

		Context("when stemcells-directory flag is specified", func() {
			It("correcty parses stemcell directory arguments", func() {
				err := bake.Execute([]string{
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
					"--stemcells-directory", "some-stemcells-directory",
					"--stemcells-directory", "some-other-stemcells-directory",
				})

				Expect(err).NotTo(HaveOccurred())

				Expect(fakeStemcellService.FromDirectoriesCallCount()).To(Equal(1))
				Expect(fakeStemcellService.FromDirectoriesArgsForCall(0)).To(Equal([]string{
					"some-stemcells-directory",
					"some-other-stemcells-directory",
				}))
			})
		})

		Context("when Kilnfile is specified", func() {
			It("renders the stemcell criteria in tile metadata from that specified the Kilnfile.lock", func() {
				outputFile := "some-output-dir/some-product-file-1.2.3-build.4"
				err := bake.Execute([]string{
					"--forms-directory", "some-forms-directory",
					"--instance-groups-directory", "some-instance-groups-directory",
					"--jobs-directory", "some-jobs-directory",
					"--metadata", "some-metadata",
					"--output-file", outputFile,
					"--properties-directory", "some-properties-directory",
					"--releases-directory", someReleasesDirectory,
					"--runtime-configs-directory", "some-other-runtime-configs-directory",
					"--kilnfile", "Kilnfile",
					"--bosh-variables-directory", "some-variables-directory",
					"--version", "1.2.3", "--migrations-directory", "some-migrations-directory",
					"--migrations-directory", "some-other-migrations-directory",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeStemcellService.FromKilnfileCallCount()).To(Equal(1))
				Expect(fakeStemcellService.FromKilnfileArgsForCall(0)).To(Equal("Kilnfile"))
				Expect(fakeFetcher.ExecuteCallCount()).To(Equal(1))
				executeArgsForFetch := fakeFetcher.ExecuteArgsForCall(0)
				Expect(executeArgsForFetch).To(Equal([]string{
					"--kilnfile", "Kilnfile",
					"--variables-file",
					"/home/.kiln/credentials.yml",
					"--download-threads", "0",
					"--no-confirm",
					"--releases-directory",
					someReleasesDirectory,
				}))
			})
		})

		Context("when neither the --kilnfile nor --stemcell-tarball flags are provided", func() {
			It("does not error", func() {
				err := bake.Execute([]string{
					"--metadata", "some-metadata",
					"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
					"--releases-directory", otherReleasesDirectory,
					"--releases-directory", someReleasesDirectory,
					"--version", "1.2.3", "--migrations-directory", "some-migrations-directory",
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("failure cases", func() {
			Context("when fetch fails", func() {
				It("returns an error", func() {
					fakeFetcher.ExecuteReturns(errors.New("fetching failed"))

					err := bake.Execute([]string{
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--releases-directory", someReleasesDirectory,
						"--kilnfile", "Kilnfile",
					})
					Expect(err).To(MatchError("fetching failed"))
				})
			})
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

			Context("when the icon service fails", func() {
				It("returns an error", func() {
					fakeIconService.EncodeReturns("", errors.New("encoding icon failed"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--releases-directory", someReleasesDirectory,
					})

					Expect(err).To(MatchError("failed to encode icon: encoding icon failed"))
				})
			})

			Context("when the metadata service fails", func() {
				It("returns an error", func() {
					fakeMetadataService.ReadReturns(nil, errors.New("reading metadata failed"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--releases-directory", someReleasesDirectory,
					})

					Expect(err).To(MatchError("failed to read metadata: reading metadata failed"))
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

			Context("when the stemcell service fails", func() {
				It("returns an error", func() {
					fakeStemcellService.FromTarballReturns(nil, errors.New("parsing stemcell failed"))

					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--properties-directory", "some-properties-directory",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("failed to parse stemcell: parsing stemcell failed"))
				})
			})

			Context("when the forms service fails", func() {
				It("returns an error", func() {
					fakeFormsService.ParseMetadataTemplatesReturns(nil, errors.New("parsing forms failed"))

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

					Expect(err).To(MatchError("failed to parse forms: parsing forms failed"))
				})
			})

			Context("when the instance groups service fails", func() {
				It("returns an error", func() {
					fakeInstanceGroupsService.ParseMetadataTemplatesReturns(nil, errors.New("parsing instance groups failed"))

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

					Expect(err).To(MatchError("failed to parse instance groups: parsing instance groups failed"))
				})
			})

			Context("when the jobs service fails", func() {
				It("returns an error", func() {
					fakeJobsService.ParseMetadataTemplatesReturns(nil, errors.New("parsing jobs failed"))

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

					Expect(err).To(MatchError("failed to parse jobs: parsing jobs failed"))
				})
			})

			Context("when the properties service fails", func() {
				It("returns an error", func() {
					fakePropertiesService.ParseMetadataTemplatesReturns(nil, errors.New("parsing properties failed"))

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

					Expect(err).To(MatchError("failed to parse properties: parsing properties failed"))
				})
			})

			Context("when the runtime configs service fails", func() {
				It("returns an error", func() {
					fakeRuntimeConfigsService.ParseMetadataTemplatesReturns(nil, errors.New("parsing runtime configs failed"))

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

					Expect(err).To(MatchError("failed to parse runtime configs: parsing runtime configs failed"))
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

			XContext("when the metadata flag is missing", func() {
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

			Context("when the release-tarball flag is missing and we are stubbing releases", func() {
				It("returns an error", func() {
					bake.Options.StubReleases = true
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4.pivotal",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--stub-releases",
						"--version", "1.2.3",
					})

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when both the --kilnfile and --stemcells-directory are provided", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--stemcells-directory", "some-stemcell-directory",
						"--kilnfile", "Kilnfile",
					})
					Expect(err).To(MatchError("--kilnfile cannot be provided when using --stemcells-directory"))
				})
			})

			// todo: When --stemcell-tarball is removed, delete this test
			Context("when both the --stemcell-tarball and --kilnfile are provided", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--kilnfile", "Kilnfile",
					})
					Expect(err).To(MatchError("--kilnfile cannot be provided when using --stemcell-tarball"))
				})
			})

			// todo: When --stemcell-tarball is remove, delete this test
			Context("when both the --stemcell-tarball and --stemcells-directory are provided", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--metadata", "some-metadata",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--stemcell-tarball", "some-stemcell-tarball",
						"--stemcells-directory", "some-stemcell-directory",
					})
					Expect(err).To(MatchError("--stemcell-tarball cannot be provided when using --stemcells-directory"))
				})
			})

			Context("when both the output-file and metadata-only flags are provided", func() {
				It("returns an error", func() {
					err := bake.Execute([]string{
						"--icon", "some-icon-path",
						"--metadata", "some-metadata",
						"--metadata-only",
						"--output-file", "some-output-dir/some-product-file-1.2.3-build.4",
						"--releases-directory", someReleasesDirectory,
						"--stemcell-tarball", "some-stemcell-tarball",
						"--version", "1.2.3",
					})

					Expect(err).To(MatchError("--output-file cannot be provided when using --metadata-only"))
				})
			})

			XContext("when the jobs-directory flag is passed without the instance-groups-directory flag", func() {
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

			Context("when the checksummer returns an error", func() {
				It("returns an error", func() {
					fakeChecksummer.SumReturns(errors.New("failed"))

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
						"--version", "1.2.3", "--migrations-directory", "some-migrations-directory",
						"--migrations-directory", "some-other-migrations-directory",
						"--variable", "some-variable=some-variable-value",
						"--variables-file", "some-variables-file",
						"--sha256",
					})

					Expect(err).To(MatchError(ContainSubstring("failed to calculate checksum: failed")))
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

var _ = Describe("BakeArgumentsFromKilnfileConfiguration", func() {
	It("handles empty options and variables", func() {
		loadKilnfile := func(s string) (cargo.Kilnfile, error) {
			return cargo.Kilnfile{}, nil
		}
		err := commands.BakeArgumentsFromKilnfileConfiguration(new(commands.BakeOptions), loadKilnfile)
		Expect(err).NotTo(HaveOccurred())
	})

	It("handles when an error occurs loading the kilnfile", func() {
		opts := &commands.BakeOptions{
			Standard: flags.Standard{
				Kilnfile: "non-empty-path/Kilnfile",
			},
		}

		loadKilnfile := func(s string) (cargo.Kilnfile, error) {
			return cargo.Kilnfile{}, os.ErrNotExist
		}

		err := commands.BakeArgumentsFromKilnfileConfiguration(opts, loadKilnfile)
		Expect(err).To(HaveOccurred())
	})

	When("passing a valid Kilnfile", func() {
		var opts *commands.BakeOptions

		const kilnfilePath = "tile/Kilnfile"

		BeforeEach(func() {
			opts = &commands.BakeOptions{
				Standard: flags.Standard{
					Kilnfile: kilnfilePath,
				},
			}
		})

		It("handles empty TileName", func() {
			var kilnfilePathArg string
			loadKilnfile := func(s string) (cargo.Kilnfile, error) {
				kilnfilePathArg = s
				return cargo.Kilnfile{}, nil
			}
			opts.TileName = ""
			err := commands.BakeArgumentsFromKilnfileConfiguration(opts, loadKilnfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(kilnfilePathArg).To(Equal(kilnfilePath))
		})

		When("there is one tile configuration", func() {
			var loadKilnfile func(string) (cargo.Kilnfile, error)
			BeforeEach(func() {
				loadKilnfile = func(s string) (cargo.Kilnfile, error) {
					return cargo.Kilnfile{
						BakeConfigurations: []cargo.BakeConfiguration{
							{TileName: "peach", Metadata: "peach.yml"},
						},
					}, nil
				}
			})
			When("tile_name is unset", func() {
				It("handles getting the first configuration", func() {
					err := commands.BakeArgumentsFromKilnfileConfiguration(opts, loadKilnfile)
					Expect(err).NotTo(HaveOccurred())
					Expect(opts.Metadata).To(Equal("peach.yml"))
				})
			})
			When("a tile_name is a variable and does not match the bake configuration", func() {
				It("handles getting the first configuration", func() {
					opts.TileName = "banana"
					err := commands.BakeArgumentsFromKilnfileConfiguration(opts, loadKilnfile)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		When("there are multiple tile configurations", func() {
			var loadKilnfile func(string) (cargo.Kilnfile, error)
			BeforeEach(func() {
				loadKilnfile = func(s string) (cargo.Kilnfile, error) {
					return cargo.Kilnfile{
						BakeConfigurations: []cargo.BakeConfiguration{
							{
								TileName: "peach",
								Metadata: "peach.yml",
							},
							{
								TileName: "pair",
								Metadata: "pair.yml",
							},
						},
					}, nil
				}
			})
			It("handles getting the first configuration by name", func() {
				opts.TileName = "pair"
				err := commands.BakeArgumentsFromKilnfileConfiguration(opts, loadKilnfile)
				Expect(err).NotTo(HaveOccurred())
				Expect(opts.Metadata).To(Equal("pair.yml"))
			})
			It("handles getting the second configuration by name", func() {
				opts.TileName = "peach"
				err := commands.BakeArgumentsFromKilnfileConfiguration(opts, loadKilnfile)
				Expect(err).NotTo(HaveOccurred())
				Expect(opts.Metadata).To(Equal("peach.yml"))
			})
			//It("handles getting the first configuration when no tile_name is passed", func() {
			//	variables := map[string]any{}
			//	err := commands.BakeArgumentsFromKilnfileConfiguration(opts, variables, loadKilnfile)
			//	Expect(err).NotTo(HaveOccurred())
			//	Expect(opts.Metadata).To(Equal("peach.yml"))
			//})
		})
	})
})

type fakeWriteBakeRecordFunc struct {
	kilnVersion, tilePath, recordPath string
	productTemplate                   []byte

	err error
}

func (f *fakeWriteBakeRecordFunc) call(kilnVersion, tilePath, recordPath string, productTemplate []byte) error {
	f.kilnVersion = kilnVersion
	f.tilePath = tilePath
	f.recordPath = recordPath
	f.productTemplate = productTemplate
	return f.err
}

func TestBakeDescription(t *testing.T) {
	o := reflect.ValueOf(commands.Bake{}.Options).Type()
	const fieldName = "IsFinal"
	field, ok := o.FieldByName(fieldName)
	if !ok {
		t.Fatalf("expected Options struct field %s", fieldName)
	}
	description := field.Tag.Get("description")
	if !strings.Contains(description, bake.RecordsDirectory) {
		t.Errorf("expected description to mention bake records directory %q", bake.RecordsDirectory)
	}
}
