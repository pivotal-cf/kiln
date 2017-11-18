package builder_test

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/builder/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetadataBuilder", func() {
	var (
		iconEncoder                   *fakes.IconEncoder
		logger                        *fakes.Logger
		metadataReader                *fakes.MetadataReader
		releaseManifestReader         *fakes.ReleaseManifestReader
		runtimeConfigsDirectoryReader *fakes.MetadataPartsDirectoryReader
		stemcellManifestReader        *fakes.StemcellManifestReader
		variablesDirectoryReader      *fakes.MetadataPartsDirectoryReader

		tileBuilder builder.MetadataBuilder
	)

	BeforeEach(func() {
		iconEncoder = &fakes.IconEncoder{}
		logger = &fakes.Logger{}
		metadataReader = &fakes.MetadataReader{}
		releaseManifestReader = &fakes.ReleaseManifestReader{}
		runtimeConfigsDirectoryReader = &fakes.MetadataPartsDirectoryReader{}
		stemcellManifestReader = &fakes.StemcellManifestReader{}
		variablesDirectoryReader = &fakes.MetadataPartsDirectoryReader{}

		iconEncoder.EncodeReturns("base64-encoded-icon-path", nil)

		releaseManifestReader.ReadStub = func(path string) (builder.ReleaseManifest, error) {
			switch path {
			case "/path/to/release-1.tgz":
				return builder.ReleaseManifest{
					Name:    "release-1",
					Version: "version-1",
				}, nil
			case "/path/to/release-2.tgz":
				return builder.ReleaseManifest{
					Name:    "release-2",
					Version: "version-2",
				}, nil
			default:
				return builder.ReleaseManifest{}, fmt.Errorf("could not read release %q", path)
			}
		}
		runtimeConfigsDirectoryReader.ReadStub = func(path string) ([]interface{}, error) {
			switch path {
			case "/path/to/runtime-configs/directory":
				return []interface{}{
					map[interface{}]interface{}{
						"name":           "runtime-config-1",
						"runtime_config": "runtime-config-1-manifest",
					},
					map[interface{}]interface{}{
						"name":           "runtime-config-2",
						"runtime_config": "runtime-config-2-manifest",
					},
				}, nil
			case "/path/to/other/runtime-configs/directory":
				return []interface{}{
					map[interface{}]interface{}{
						"name":           "runtime-config-3",
						"runtime_config": "runtime-config-3-manifest",
					},
				}, nil
			default:
				return []interface{}{}, fmt.Errorf("could not read runtime configs directory %q", path)
			}
		}

		variablesDirectoryReader.ReadStub = func(path string) ([]interface{}, error) {
			switch path {
			case "/path/to/variables/directory":
				return []interface{}{
					map[interface{}]interface{}{
						"name": "variable-1",
						"type": "certificate",
					},
					map[interface{}]interface{}{
						"name": "variable-2",
						"type": "user",
					},
				}, nil
			case "/path/to/other/variables/directory":
				return []interface{}{
					map[interface{}]interface{}{
						"name": "variable-3",
						"type": "password",
					},
				}, nil
			default:
				return []interface{}{}, fmt.Errorf("could not read variables directory %q", path)
			}
		}
		stemcellManifestReader.ReadReturns(builder.StemcellManifest{
			Version:         "2332",
			OperatingSystem: "ubuntu-trusty",
		},
			nil,
		)

		tileBuilder = builder.NewMetadataBuilder(
			releaseManifestReader,
			runtimeConfigsDirectoryReader,
			variablesDirectoryReader,
			stemcellManifestReader,
			metadataReader,
			logger,
			iconEncoder,
		)
	})

	Describe("Build", func() {
		BeforeEach(func() {
			metadataReader.ReadReturns(builder.Metadata{
				"name":                      "cool-product",
				"metadata_version":          "some-metadata-version",
				"provides_product_versions": "some-provides-product-versions",
				"icon_image":                "unused-icon-image-IGNORE-ME",
			},
				nil,
			)
		})

		It("creates a GeneratedMetadata with the correct information", func() {
			generatedMetadata, err := tileBuilder.Build(
				[]string{"/path/to/release-1.tgz", "/path/to/release-2.tgz"},
				[]string{"/path/to/runtime-configs/directory", "/path/to/other/runtime-configs/directory"},
				[]string{"/path/to/variables/directory", "/path/to/other/variables/directory"},
				"/path/to/test-stemcell.tgz",
				"/some/path/metadata.yml",
				"1.2.3",
				"/path/to/tile.zip",
				"some-icon-path",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(stemcellManifestReader.ReadArgsForCall(0)).To(Equal("/path/to/test-stemcell.tgz"))
			metadataPath, version := metadataReader.ReadArgsForCall(0)
			Expect(metadataPath).To(Equal("/some/path/metadata.yml"))
			Expect(version).To(Equal("1.2.3"))

			Expect(generatedMetadata.Name).To(Equal("cool-product"))
			Expect(generatedMetadata.Releases).To(Equal([]builder.Release{
				{
					Name:    "release-1",
					Version: "version-1",
					File:    "release-1.tgz",
				},
				{
					Name:    "release-2",
					Version: "version-2",
					File:    "release-2.tgz",
				},
			}))
			Expect(generatedMetadata.RuntimeConfigs).To(Equal([]interface{}{
				map[interface{}]interface{}{
					"name":           "runtime-config-1",
					"runtime_config": "runtime-config-1-manifest",
				},
				map[interface{}]interface{}{
					"name":           "runtime-config-2",
					"runtime_config": "runtime-config-2-manifest",
				},
				map[interface{}]interface{}{
					"name":           "runtime-config-3",
					"runtime_config": "runtime-config-3-manifest",
				},
			},
			))
			Expect(generatedMetadata.Variables).To(Equal([]interface{}{
				map[interface{}]interface{}{
					"name": "variable-1",
					"type": "certificate",
				},
				map[interface{}]interface{}{
					"name": "variable-2",
					"type": "user",
				},
				map[interface{}]interface{}{
					"name": "variable-3",
					"type": "password",
				},
			}))
			Expect(generatedMetadata.StemcellCriteria).To(Equal(builder.StemcellCriteria{
				Version:     "2332",
				OS:          "ubuntu-trusty",
				RequiresCPI: false,
			}))
			Expect(generatedMetadata.Metadata).To(Equal(builder.Metadata{
				"metadata_version":          "some-metadata-version",
				"provides_product_versions": "some-provides-product-versions",
			}))

			Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
				"Creating metadata for /path/to/tile.zip...",
				"Read manifest for release release-1",
				"Read manifest for release release-2",
				"Read runtime configs from /path/to/runtime-configs/directory",
				"Read runtime configs from /path/to/other/runtime-configs/directory",
				"Read variables from /path/to/variables/directory",
				"Read variables from /path/to/other/variables/directory",
				"Read manifest for stemcell version 2332",
				"Read metadata",
			}))

			Expect(iconEncoder.EncodeCallCount()).To(Equal(1))
			Expect(iconEncoder.EncodeArgsForCall(0)).To(Equal("some-icon-path"))

			Expect(generatedMetadata.IconImage).To(Equal("base64-encoded-icon-path"))
		})

		Context("failure cases", func() {
			Context("when the release tarball cannot be read", func() {
				It("returns an error", func() {
					releaseManifestReader.ReadReturns(builder.ReleaseManifest{}, errors.New("failed to read release tarball"))

					_, err := tileBuilder.Build([]string{"release-1.tgz"}, []string{}, []string{}, "", "", "", "", "")
					Expect(err).To(MatchError("failed to read release tarball"))
				})
			})

			Context("when the runtime configs directory cannot be read", func() {
				It("returns an error", func() {
					runtimeConfigsDirectoryReader.ReadReturns([]interface{}{}, errors.New("some error"))

					_, err := tileBuilder.Build([]string{}, []string{"/path/to/missing/runtime-configs"}, []string{}, "", "", "", "", "")
					Expect(err).To(MatchError(`error reading from runtime configs directory "/path/to/missing/runtime-configs": some error`))
				})
			})

			Context("when the variables directory cannot be read", func() {
				It("returns an error", func() {
					variablesDirectoryReader.ReadReturns([]interface{}{}, errors.New("some error"))

					_, err := tileBuilder.Build([]string{}, []string{}, []string{"/path/to/missing/variables"}, "", "", "", "", "")
					Expect(err).To(MatchError(`error reading from variables directory "/path/to/missing/variables": some error`))
				})
			})

			Context("when the stemcell tarball cannot be read", func() {
				It("returns an error", func() {
					stemcellManifestReader.ReadReturns(builder.StemcellManifest{}, errors.New("failed to read stemcell tarball"))

					_, err := tileBuilder.Build([]string{}, []string{}, []string{}, "stemcell.tgz", "", "", "", "")
					Expect(err).To(MatchError("failed to read stemcell tarball"))
				})
			})

			Context("when the icon cannot be encoded", func() {
				BeforeEach(func() {
					iconEncoder.EncodeReturns("", errors.New("failed to encode poncho"))
				})

				It("returns an error", func() {
					_, err := tileBuilder.Build([]string{}, []string{}, []string{}, "stemcell.tgz", "", "", "", "")
					Expect(err).To(MatchError("failed to encode poncho"))
				})
			})

			Context("when the metadata cannot be read", func() {
				It("returns an error", func() {
					metadataReader.ReadReturns(builder.Metadata{}, errors.New("failed to read metadata"))

					_, err := tileBuilder.Build([]string{}, []string{}, []string{}, "", "metadata.yml", "", "", "")
					Expect(err).To(MatchError("failed to read metadata"))
				})
			})

			Context("when the metadata does not contain a product name", func() {
				It("returns an error", func() {
					metadataReader.ReadReturns(builder.Metadata{
						"metadata_version":          "some-metadata-version",
						"provides_product_versions": "some-provides-product-versions",
					},
						nil,
					)

					_, err := tileBuilder.Build([]string{}, []string{}, []string{}, "", "metadata.yml", "", "", "")
					Expect(err).To(MatchError(`missing "name" in tile metadata`))
				})
			})

			Context("when the base metadata contains a runtime_configs section", func() {
				It("returns an error", func() {
					metadataReader.ReadReturns(builder.Metadata{
						"name":            "cool-product",
						"runtime_configs": "some-runtime-configs",
					},
						nil,
					)
					_, err := tileBuilder.Build([]string{}, []string{}, []string{}, "", "metadata.yml", "", "", "")
					Expect(err).To(MatchError(`runtime_config section must be defined using --runtime-configs-directory flag, not in "metadata.yml"`))
				})
			})

			Context("when the base metadata contains a variables section", func() {
				It("returns an error", func() {
					metadataReader.ReadReturns(builder.Metadata{
						"name":      "cool-product",
						"variables": "some-variables",
					},
						nil,
					)
					_, err := tileBuilder.Build([]string{}, []string{}, []string{}, "", "metadata.yml", "", "", "")
					Expect(err).To(MatchError(`variables section must be defined using --variables-directory flag, not in "metadata.yml"`))
				})
			})
		})
	})
})
