package builder_test

import (
	"errors"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/builder/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetadataBuilder", func() {
	var (
		logger         *fakes.Logger
		metadataReader *fakes.MetadataReader

		tileBuilder builder.MetadataBuilder
	)

	BeforeEach(func() {
		logger = &fakes.Logger{}
		metadataReader = &fakes.MetadataReader{}

		tileBuilder = builder.NewMetadataBuilder(
			metadataReader,
			logger,
		)
	})

	Describe("Build", func() {
		BeforeEach(func() {
			metadataReader.ReadReturns(builder.Metadata{
				"metadata_version":          "some-metadata-version",
				"provides_product_versions": "some-provides-product-versions",
			},
				nil,
			)
		})

		It("creates a GeneratedMetadata with the correct information", func() {
			generatedMetadata, err := tileBuilder.Build(builder.BuildInput{
				MetadataPath: "/some/path/metadata.yml",
				Version:      "1.2.3",
			})
			Expect(err).NotTo(HaveOccurred())
			metadataPath, version := metadataReader.ReadArgsForCall(0)
			Expect(metadataPath).To(Equal("/some/path/metadata.yml"))
			Expect(version).To(Equal("1.2.3"))

			Expect(generatedMetadata.Metadata).To(Equal(builder.Metadata{
				"metadata_version":          "some-metadata-version",
				"provides_product_versions": "some-provides-product-versions",
			}))
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
			Context("when the metadata cannot be read", func() {
				It("returns an error", func() {
					metadataReader.ReadReturns(builder.Metadata{}, errors.New("failed to read metadata"))

					_, err := tileBuilder.Build(builder.BuildInput{
						MetadataPath: "metadata.yml",
					})
					Expect(err).To(MatchError("failed to read metadata"))
				})
			})
		})
	})
})
