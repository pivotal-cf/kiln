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
		instanceGroupsDirectoryReader *fakes.MetadataPartsDirectoryReader
		jobsDirectoryReader           *fakes.MetadataPartsDirectoryReader
		logger                        *fakes.Logger
		metadataReader                *fakes.MetadataReader
		propertiesDirectoryReader     *fakes.MetadataPartsDirectoryReader
		runtimeConfigsDirectoryReader *fakes.MetadataPartsDirectoryReader
		variablesDirectoryReader      *fakes.MetadataPartsDirectoryReader

		tileBuilder builder.MetadataBuilder
	)

	BeforeEach(func() {
		iconEncoder = &fakes.IconEncoder{}
		instanceGroupsDirectoryReader = &fakes.MetadataPartsDirectoryReader{}
		jobsDirectoryReader = &fakes.MetadataPartsDirectoryReader{}
		logger = &fakes.Logger{}
		metadataReader = &fakes.MetadataReader{}
		propertiesDirectoryReader = &fakes.MetadataPartsDirectoryReader{}
		runtimeConfigsDirectoryReader = &fakes.MetadataPartsDirectoryReader{}
		variablesDirectoryReader = &fakes.MetadataPartsDirectoryReader{}

		iconEncoder.EncodeReturns("base64-encoded-icon-path", nil)

		instanceGroupsDirectoryReader.ReadStub = func(path string) ([]builder.Part, error) {
			switch path {
			case "/path/to/instance-groups/directory":
				return []builder.Part{
					{
						File: "some-instance-group-1.yml",
						Name: "some-instance-group-1",
						Metadata: map[interface{}]interface{}{
							"name": "some-instance-group-1",
						},
					},
					{
						File: "some-instance-group-2.yml",
						Name: "some-instance-group-2",
						Metadata: map[interface{}]interface{}{
							"name": "some-instance-group-2",
						},
					},
				}, nil
			default:
				return []builder.Part{}, fmt.Errorf("could not read instance groups directory %q", path)
			}
		}

		jobsDirectoryReader.ReadStub = func(path string) ([]builder.Part, error) {
			switch path {
			case "/path/to/jobs/directory":
				return []builder.Part{
						{
							File: "some-job-1.yml",
							Name: "some-job-1",
							Metadata: map[interface{}]interface{}{
								"name":    "some-job-1",
								"release": "some-release-1",
							},
						},
						{
							File: "some-job-2.yml",
							Name: "some-job-2",
							Metadata: map[interface{}]interface{}{
								"name":    "some-job-2",
								"release": "some-release-2",
							},
						},
					},
					nil
			default:
				return []builder.Part{}, fmt.Errorf("could not read instance groups directory %q", path)
			}
		}

		propertiesDirectoryReader.ReadStub = func(path string) ([]builder.Part, error) {
			switch path {
			case "/path/to/properties/directory":
				return []builder.Part{
					{
						File: "property-1.yml",
						Name: "property-1",
						Metadata: map[interface{}]interface{}{
							"name": "property-1",
						},
					},
					{
						File: "property-2.yml",
						Name: "property-2",
						Metadata: map[interface{}]interface{}{
							"name": "property-2",
						},
					},
				}, nil
			default:
				return []builder.Part{}, fmt.Errorf("could not read properties directory %q", path)
			}
		}

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
			instanceGroupsDirectoryReader,
			jobsDirectoryReader,
			propertiesDirectoryReader,
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
				"job_types":                 "job-types-to-be-overridden",
			},
				nil,
			)
		})

		It("creates a GeneratedMetadata with the correct information", func() {
			generatedMetadata, err := tileBuilder.Build(builder.BuildInput{
				IconPath:                 "some-icon-path",
				InstanceGroupDirectories: []string{"/path/to/instance-groups/directory"},
				JobDirectories:           []string{"/path/to/jobs/directory"},
				MetadataPath:             "/some/path/metadata.yml",
				PropertyDirectories:      []string{"/path/to/properties/directory"},
				RuntimeConfigDirectories: []string{"/path/to/runtime-configs/directory", "/path/to/other/runtime-configs/directory"},
				BOSHVariableDirectories:  []string{"/path/to/variables/directory", "/path/to/other/variables/directory"},
				Version:                  "1.2.3",
			})
			Expect(err).NotTo(HaveOccurred())
			metadataPath, version := metadataReader.ReadArgsForCall(0)
			Expect(metadataPath).To(Equal("/some/path/metadata.yml"))
			Expect(version).To(Equal("1.2.3"))

			Expect(generatedMetadata.Name).To(Equal("cool-product"))
			Expect(generatedMetadata.JobTypes).To(Equal([]builder.Part{
				{
					File: "some-instance-group-1.yml",
					Name: "some-instance-group-1",
					Metadata: map[interface{}]interface{}{
						"name": "some-instance-group-1",
					},
				},
				{
					File: "some-instance-group-2.yml",
					Name: "some-instance-group-2",
					Metadata: map[interface{}]interface{}{
						"name": "some-instance-group-2",
					},
				},
			}))
			Expect(generatedMetadata.PropertyBlueprints).To(Equal([]builder.Part{
				{
					File: "property-1.yml",
					Name: "property-1",
					Metadata: map[interface{}]interface{}{
						"name": "property-1",
					},
				},
				{
					File: "property-2.yml",
					Name: "property-2",
					Metadata: map[interface{}]interface{}{
						"name": "property-2",
					},
				},
			}))
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
				"Reading instance groups from /path/to/instance-groups/directory",
				"Reading property blueprints from /path/to/properties/directory",
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

			It("includes the property blueprints from the metadata", func() {
				generatedMetadata, err := tileBuilder.Build(builder.BuildInput{
					MetadataPath:    "/some/path/metadata.yml",
					FormDirectories: []string{},
					IconPath:        "some-icon-path",
					Version:         "1.2.3",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(generatedMetadata.PropertyBlueprints).To(Equal([]builder.Part{
					{
						Metadata: map[interface{}]interface{}{
							"name": "property-1",
							"type": "string",
						},
					},
				}))
			})
		})

		Context("when no job directories are specified", func() {
			BeforeEach(func() {
				metadataReader.ReadReturns(builder.Metadata{
					"name":                      "cool-product",
					"metadata_version":          "some-metadata-version",
					"provides_product_versions": "some-provides-product-versions",
					"job_types": []interface{}{
						map[interface{}]interface{}{
							"name":  "job-type",
							"label": "Job Type",
						},
					},
				},
					nil,
				)
			})

			It("includes the job types from the metadata", func() {
				generatedMetadata, err := tileBuilder.Build(builder.BuildInput{
					MetadataPath:   "/some/path/metadata.yml",
					JobDirectories: []string{},
					IconPath:       "some-icon-path",
					Version:        "1.2.3",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(generatedMetadata.JobTypes).To(Equal([]builder.Part{
					{
						Metadata: map[interface{}]interface{}{
							"name":  "job-type",
							"label": "Job Type",
						},
					},
				}))
			})
		})

		Context("failure cases", func() {
			Context("when the properties directory cannot be read", func() {
				It("returns an error", func() {
					propertiesDirectoryReader.ReadReturns([]builder.Part{}, errors.New("some properties error"))

					_, err := tileBuilder.Build(builder.BuildInput{
						PropertyDirectories: []string{"/path/to/missing/property"},
					})
					Expect(err).To(MatchError(`error reading from properties directory "/path/to/missing/property": some properties error`))
				})
			})

			Context("when the instance group directory cannot be read", func() {
				It("returns an error", func() {
					instanceGroupsDirectoryReader.ReadReturns([]builder.Part{}, errors.New("some instance group error"))

					_, err := tileBuilder.Build(builder.BuildInput{
						InstanceGroupDirectories: []string{"/path/to/missing/instance-groups"},
					})
					Expect(err).To(MatchError(`error reading from instance group directory "/path/to/missing/instance-groups": some instance group error`))
				})
			})

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
