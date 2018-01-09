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
		runtimeConfigsDirectoryReader *fakes.MetadataPartsDirectoryReader
		variablesDirectoryReader      *fakes.MetadataPartsDirectoryReader

		tileBuilder builder.MetadataBuilder
	)

	BeforeEach(func() {
		iconEncoder = &fakes.IconEncoder{}
		logger = &fakes.Logger{}
		metadataReader = &fakes.MetadataReader{}
		runtimeConfigsDirectoryReader = &fakes.MetadataPartsDirectoryReader{}
		variablesDirectoryReader = &fakes.MetadataPartsDirectoryReader{}

		iconEncoder.EncodeReturns("base64-encoded-icon-path", nil)

		runtimeConfigsDirectoryReader.ReadStub = func(path string) ([]builder.Part, error) {
			switch path {
			case "/path/to/runtime-configs/directory":
				return []builder.Part{
					{
						File: "runtime-config-1.yml",
						Name: "runtime-config-1",
						Metadata: map[interface{}]interface{}{
							"name":           "runtime-config-1",
							"runtime_config": "runtime-config-1-manifest",
						},
					},
					{
						File: "runtime-config-2.yml",
						Name: "runtime-config-2",
						Metadata: map[interface{}]interface{}{
							"name":           "runtime-config-2",
							"runtime_config": "runtime-config-2-manifest",
						},
					},
				}, nil
			case "/path/to/other/runtime-configs/directory":
				return []builder.Part{
					{
						File: "runtime-config-3.yml",
						Name: "runtime-config-3",
						Metadata: map[interface{}]interface{}{
							"name":           "runtime-config-3",
							"runtime_config": "runtime-config-3-manifest",
						},
					},
				}, nil
			default:
				return []builder.Part{}, fmt.Errorf("could not read runtime configs directory %q", path)
			}
		}

		variablesDirectoryReader.ReadStub = func(path string) ([]builder.Part, error) {
			switch path {
			case "/path/to/variables/directory":
				return []builder.Part{
					{
						File: "variable-1.yml",
						Name: "variable-1",
						Metadata: map[interface{}]interface{}{
							"name": "variable-1",
							"type": "certificate",
						},
					},
					{
						File: "variable-2.yml",
						Name: "variable-2",
						Metadata: map[interface{}]interface{}{
							"name": "variable-2",
							"type": "user",
						},
					},
				}, nil
			case "/path/to/other/variables/directory":
				return []builder.Part{
					{
						File: "variable-3.yml",
						Name: "variable-3",
						Metadata: map[interface{}]interface{}{
							"name": "variable-3",
							"type": "password",
						},
					},
				}, nil
			default:
				return []builder.Part{}, fmt.Errorf("could not read variables directory %q", path)
			}
		}

		tileBuilder = builder.NewMetadataBuilder(
			runtimeConfigsDirectoryReader,
			variablesDirectoryReader,
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
			},
				nil,
			)
		})

		It("creates a GeneratedMetadata with the correct information", func() {
			generatedMetadata, err := tileBuilder.Build(builder.BuildInput{
				IconPath:                 "some-icon-path",
				MetadataPath:             "/some/path/metadata.yml",
				RuntimeConfigDirectories: []string{"/path/to/runtime-configs/directory", "/path/to/other/runtime-configs/directory"},
				BOSHVariableDirectories:  []string{"/path/to/variables/directory", "/path/to/other/variables/directory"},
				Version:                  "1.2.3",
			})
			Expect(err).NotTo(HaveOccurred())
			metadataPath, version := metadataReader.ReadArgsForCall(0)
			Expect(metadataPath).To(Equal("/some/path/metadata.yml"))
			Expect(version).To(Equal("1.2.3"))

			Expect(generatedMetadata.Name).To(Equal("cool-product"))
			Expect(generatedMetadata.RuntimeConfigs).To(Equal([]builder.Part{
				{
					File: "runtime-config-1.yml",
					Name: "runtime-config-1",
					Metadata: map[interface{}]interface{}{
						"name":           "runtime-config-1",
						"runtime_config": "runtime-config-1-manifest",
					},
				},
				{
					File: "runtime-config-2.yml",
					Name: "runtime-config-2",
					Metadata: map[interface{}]interface{}{
						"name":           "runtime-config-2",
						"runtime_config": "runtime-config-2-manifest",
					},
				},
				{
					File: "runtime-config-3.yml",
					Name: "runtime-config-3",
					Metadata: map[interface{}]interface{}{
						"name":           "runtime-config-3",
						"runtime_config": "runtime-config-3-manifest",
					},
				},
			}))
			Expect(generatedMetadata.Variables).To(Equal([]builder.Part{
				{
					File: "variable-1.yml",
					Name: "variable-1",
					Metadata: map[interface{}]interface{}{
						"name": "variable-1",
						"type": "certificate",
					},
				},
				{
					File: "variable-2.yml",
					Name: "variable-2",
					Metadata: map[interface{}]interface{}{
						"name": "variable-2",
						"type": "user",
					},
				},
				{
					File: "variable-3.yml",
					Name: "variable-3",
					Metadata: map[interface{}]interface{}{
						"name": "variable-3",
						"type": "password",
					},
				},
			}))
			Expect(generatedMetadata.Metadata).To(Equal(builder.Metadata{
				"metadata_version":          "some-metadata-version",
				"provides_product_versions": "some-provides-product-versions",
			}))

			Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
				"Reading runtime configs from /path/to/runtime-configs/directory",
				"Reading runtime configs from /path/to/other/runtime-configs/directory",
				"Reading variables from /path/to/variables/directory",
				"Reading variables from /path/to/other/variables/directory",
			}))

			Expect(iconEncoder.EncodeCallCount()).To(Equal(1))
			Expect(iconEncoder.EncodeArgsForCall(0)).To(Equal("some-icon-path"))

			Expect(generatedMetadata.IconImage).To(Equal("base64-encoded-icon-path"))
		})

		Context("when no property directories are specified", func() {
			BeforeEach(func() {
				metadataReader.ReadReturns(builder.Metadata{
					"name":                      "cool-product",
					"metadata_version":          "some-metadata-version",
					"provides_product_versions": "some-provides-product-versions",
					"property_blueprints": []interface{}{
						map[interface{}]interface{}{
							"name": "property-1",
							"type": "string",
						},
					},
				},
					nil,
				)
			})
		})

		Context("failure cases", func() {
			Context("when the runtime configs directory cannot be read", func() {
				It("returns an error", func() {
					runtimeConfigsDirectoryReader.ReadReturns([]builder.Part{}, errors.New("some error"))

					_, err := tileBuilder.Build(builder.BuildInput{
						RuntimeConfigDirectories: []string{"/path/to/missing/runtime-configs"},
					})
					Expect(err).To(MatchError(`error reading from runtime configs directory "/path/to/missing/runtime-configs": some error`))
				})
			})

			Context("when the variables directory cannot be read", func() {
				It("returns an error", func() {
					variablesDirectoryReader.ReadReturns([]builder.Part{}, errors.New("some error"))

					_, err := tileBuilder.Build(builder.BuildInput{
						BOSHVariableDirectories: []string{"/path/to/missing/variables"},
					})
					Expect(err).To(MatchError(`error reading from variables directory "/path/to/missing/variables": some error`))
				})
			})

			Context("when the icon cannot be encoded", func() {
				BeforeEach(func() {
					iconEncoder.EncodeReturns("", errors.New("failed to encode poncho"))
				})

				It("returns an error", func() {
					_, err := tileBuilder.Build(builder.BuildInput{
						IconPath: "some-icon-path",
					})
					Expect(err).To(MatchError("failed to encode poncho"))
				})
			})

			Context("when the metadata cannot be read", func() {
				It("returns an error", func() {
					metadataReader.ReadReturns(builder.Metadata{}, errors.New("failed to read metadata"))

					_, err := tileBuilder.Build(builder.BuildInput{
						MetadataPath: "metadata.yml",
					})
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

					_, err := tileBuilder.Build(builder.BuildInput{
						MetadataPath: "metadata.yml",
					})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(`missing "name" in tile metadata`))
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

					_, err := tileBuilder.Build(builder.BuildInput{
						MetadataPath: "metadata.yml",
					})
					Expect(err).To(MatchError("runtime_config section must be defined using --runtime-configs-directory flag"))
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

					_, err := tileBuilder.Build(builder.BuildInput{
						MetadataPath: "metadata.yml",
					})
					Expect(err).To(MatchError("variables section must be defined using --variables-directory flag"))
				})
			})
		})
	})
})
