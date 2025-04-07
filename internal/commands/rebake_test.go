package commands_test

//
//import (
//	. "github.com/onsi/ginkgo"
//	. "github.com/onsi/gomega"
//	"github.com/pivotal-cf/jhanda"
//	"github.com/pivotal-cf/kiln/internal/builder"
//	"github.com/pivotal-cf/kiln/internal/commands"
//	"github.com/pivotal-cf/kiln/internal/commands/fakes"
//	"github.com/pivotal-cf/kiln/pkg/cargo"
//	"github.com/pivotal-cf/kiln/pkg/proofing"
//	"log"
//	"os"
//	"path/filepath"
//)
//
//var _ = Describe("Re-Bake", func() {
//	var (
//		fakeInterpolator *fakes.Interpolator
//		fakeLogger       *log.Logger
//
//		fakeIconService     *fakes.IconService
//		fakeStemcellService *fakes.StemcellService
//		fakeReleasesService *fakes.FromDirectories
//		fakeFetcher         *fakes.Fetch
//
//		fakeTemplateVariablesService *fakes.TemplateVariablesService
//		fakeMetadataService          *fakes.MetadataService
//
//		fakeBOSHVariablesService,
//		fakeFormsService,
//		fakeInstanceGroupsService,
//		fakeJobsService,
//		fakePropertiesService,
//		fakeRuntimeConfigsService *fakes.MetadataTemplatesParser
//
//		fakeTileWriter  *fakes.TileWriter
//		fakeChecksummer *fakes.Checksummer
//
//		fakeFilesystem  *fakes.FileSystem
//		fakeHomeDirFunc func() (string, error)
//
//		someTileOutputDirectory   string
//		someBakedRecordsDirectory string
//		bakedRecordContents       string
//		someBakedRecords          string
//		tmpDir                    string
//
//		fakeBakeRecordFunc *fakeWriteBakeRecordFunc
//
//		bake   commands.Bake
//		rebake commands.ReBake
//	)
//
//	BeforeEach(func() {
//		var err error
//		tmpDir, err = os.MkdirTemp("", "rebake-command-test")
//		Expect(err).NotTo(HaveOccurred())
//
//		someTileOutputDirectory, err = os.MkdirTemp(tmpDir, "tile")
//		someBakedRecordsDirectory, err = os.MkdirTemp(tmpDir, "baked_records")
//		Expect(err).NotTo(HaveOccurred())
//
//		bakedRecordContents = `
//	{
//		"source_revision": "some-revision",
//		"version": "3.3.0",
//		"kiln_version": "0.94.0",
//		"file_checksum": "some-checksum",
//		"tile_directory": "."
//	}
//	`
//
//		someBakedRecords = filepath.Join(someBakedRecordsDirectory, "3.3.0.json")
//
//		err = os.WriteFile(someBakedRecords, []byte(bakedRecordContents), 0o644)
//		Expect(err).NotTo(HaveOccurred())
//
//		fakeTileWriter = &fakes.TileWriter{}
//		fakeChecksummer = &fakes.Checksummer{}
//		fakeIconService = &fakes.IconService{}
//		fakeInterpolator = &fakes.Interpolator{}
//		fakeBakeRecordFunc = &fakeWriteBakeRecordFunc{}
//
//		fakeLogger = log.New(GinkgoWriter, "", 0)
//
//		fakeStemcellService = &fakes.StemcellService{}
//		fakeReleasesService = &fakes.FromDirectories{}
//
//		fakeTemplateVariablesService = &fakes.TemplateVariablesService{}
//		fakeMetadataService = &fakes.MetadataService{}
//		fakeInstanceGroupsService = &fakes.MetadataTemplatesParser{}
//		fakeBOSHVariablesService = &fakes.MetadataTemplatesParser{}
//		fakeFormsService = &fakes.MetadataTemplatesParser{}
//		fakeJobsService = &fakes.MetadataTemplatesParser{}
//		fakePropertiesService = &fakes.MetadataTemplatesParser{}
//		fakeRuntimeConfigsService = &fakes.MetadataTemplatesParser{}
//		fakeFilesystem = &fakes.FileSystem{}
//		fakeVersionInfo := &fakes.FileInfo{}
//		fileVersion := "some-version"
//		fakeVersionInfo.SizeReturns(int64(len(fileVersion)))
//		fakeVersionInfo.NameReturns("version")
//		fakeFilesystem.StatReturns(fakeVersionInfo, nil)
//		result1 := &fakes.File{}
//		result1.ReadReturns(0, nil)
//		fakeFilesystem.OpenReturns(result1, nil)
//		fakeHomeDirFunc = func() (string, error) {
//			return "/home/", nil
//		}
//
//		fakeTemplateVariablesService.FromPathsAndPairsReturns(map[string]any{
//			"some-variable-from-file": "some-variable-value-from-file",
//			"some-variable":           "some-variable-value",
//		}, nil)
//
//		fakeReleasesService.FromDirectoriesReturns(map[string]any{
//			"some-release-1": proofing.Release{
//				Name:    "some-release-1",
//				Version: "1.2.3",
//				File:    "release1.tgz",
//			},
//			"some-release-2": proofing.Release{
//				Name:    "some-release-2",
//				Version: "2.3.4",
//				File:    "release2.tar.gz",
//			},
//		}, nil)
//
//		fakeStemcellService.FromTarballReturns(builder.StemcellManifest{
//			Version:         "2.3.4",
//			OperatingSystem: "an-operating-system",
//		}, nil)
//
//		fakeFormsService.ParseMetadataTemplatesReturns(map[string]any{
//			"some-form": builder.Metadata{
//				"name":  "some-form",
//				"label": "some-form-label",
//			},
//		}, nil)
//
//		fakeBOSHVariablesService.ParseMetadataTemplatesReturns(map[string]any{
//			"some-secret": builder.Metadata{
//				"name": "some-secret",
//				"type": "password",
//			},
//		}, nil)
//
//		fakeInstanceGroupsService.ParseMetadataTemplatesReturns(map[string]any{
//			"some-instance-group": builder.Metadata{
//				"name":     "some-instance-group",
//				"manifest": "some-manifest",
//				"provides": "some-link",
//				"release":  "some-release",
//			},
//		}, nil)
//
//		fakeJobsService.ParseMetadataTemplatesReturns(map[string]any{
//			"some-job": builder.Metadata{
//				"name":     "some-job",
//				"release":  "some-release",
//				"consumes": "some-link",
//			},
//		}, nil)
//
//		fakePropertiesService.ParseMetadataTemplatesReturns(map[string]any{
//			"some-property": builder.Metadata{
//				"name":         "some-property",
//				"type":         "boolean",
//				"configurable": true,
//				"default":      false,
//			},
//		}, nil)
//
//		fakeRuntimeConfigsService.ParseMetadataTemplatesReturns(map[string]any{
//			"some-runtime-config": builder.Metadata{
//				"name":           "some-runtime-config",
//				"runtime_config": "some-addon-runtime-config",
//			},
//		}, nil)
//
//		fakeIconService.EncodeReturns("some-encoded-icon", nil)
//
//		fakeMetadataService.ReadReturns([]byte("some-metadata"), nil)
//
//		fakeInterpolator.InterpolateReturns([]byte("some-interpolated-metadata"), nil)
//
//		fakeFetcher = &fakes.Fetch{}
//		fakeFetcher.ExecuteReturns(nil)
//		bake = commands.NewBakeWithInterfaces(fakeInterpolator, fakeTileWriter, fakeLogger, fakeLogger, fakeTemplateVariablesService, fakeBOSHVariablesService, fakeReleasesService, fakeStemcellService, fakeFormsService, fakeInstanceGroupsService, fakeJobsService, fakePropertiesService, fakeRuntimeConfigsService, fakeIconService, fakeMetadataService, fakeChecksummer, fakeFetcher, fakeFilesystem, fakeHomeDirFunc, fakeBakeRecordFunc.call)
//		bake = bake.WithKilnfileFunc(func(s string) (cargo.Kilnfile, error) { return cargo.Kilnfile{}, nil })
//	})
//
//	AfterEach(func() {
//		Expect(os.RemoveAll(tmpDir)).To(Succeed())
//	})
//
//	Describe("Execute", func() {
//		It("re-builds the tile", func() {
//			err := rebake.Execute([]string{
//				"--output-file", someTileOutputDirectory,
//				someBakedRecords,
//			})
//			Expect(err).NotTo(HaveOccurred())
//		})
//
//	})
//
//	Describe("Usage", func() {
//		It("returns usage information for the command", func() {
//			Expect(rebake.Usage()).To(Equal(jhanda.Usage{
//				Description:      "re-bake (aka record bake) builds a tile from a bake record. You must check out the repository to the revision of the source_revision in the bake record before running this command.",
//				ShortDescription: "re-bake constructs a tile from a bake record",
//				Flags:            rebake.Options,
//			}))
//		})
//	})
//})
