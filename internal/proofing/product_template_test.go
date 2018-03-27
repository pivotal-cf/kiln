package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProductTemplate", func() {
	var productTemplate proofing.ProductTemplate

	BeforeEach(func() {
		var err error
		productTemplate, err = proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())
	})

	It("parses a metadata file", func() {
		Expect(productTemplate.IconImage).To(Equal("some-icon-image"))
		Expect(productTemplate.Label).To(Equal("some-label"))
		Expect(productTemplate.MetadataVersion).To(Equal("some-metadata-version"))
		Expect(productTemplate.MinimumVersionForUpgrade).To(Equal("some-minimum-version-for-upgrade"))
		Expect(productTemplate.Name).To(Equal("some-name"))
		Expect(productTemplate.ProductVersion).To(Equal("some-product-version"))
		Expect(productTemplate.Rank).To(Equal(1))
		Expect(productTemplate.Serial).To(BeTrue())
		Expect(productTemplate.OriginalMetadataVersion).To(Equal("some-original-metadata-version"))
		Expect(productTemplate.ServiceBroker).To(BeTrue())
		Expect(productTemplate.DeprecatedTileImage).To(Equal("some-deprecated-tile-image"))
		Expect(productTemplate.BaseReleasesURL).To(Equal("some-base-releases-url"))
		Expect(productTemplate.Cloud).To(Equal("some-cloud"))
		Expect(productTemplate.Network).To(Equal("some-network"))

		Expect(productTemplate.FormTypes).To(HaveLen(1))
		Expect(productTemplate.InstallTimeVerifiers).To(HaveLen(1))
		Expect(productTemplate.JobTypes).To(HaveLen(1))
		Expect(productTemplate.PostDeployErrands).To(HaveLen(1))
		Expect(productTemplate.PreDeleteErrands).To(HaveLen(1))
		Expect(productTemplate.PropertyBlueprints).To(HaveLen(1))
		Expect(productTemplate.RequiresProductVersions).To(HaveLen(1))
		Expect(productTemplate.Releases).To(HaveLen(1))
		Expect(productTemplate.RuntimeConfigs).To(HaveLen(1))
		Expect(productTemplate.StemcellCriteria).To(BeAssignableToTypeOf(proofing.StemcellCriteria{}))
		Expect(productTemplate.Variables).To(HaveLen(1))
	})

	Describe("AllPropertyBlueprints", func() {
		BeforeEach(func() {
			var err error
			productTemplate, err = proofing.Parse("fixtures/property_blueprints.yml")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns all property blueprints as a list", func() {
			propertyBlueprints := productTemplate.AllPropertyBlueprints()

			Expect(propertyBlueprints).To(HaveLen(6))

			Expect(propertyBlueprints).To(HaveKey(".properties.some-simple-name"))
			simplePB, ok := propertyBlueprints[".properties.some-simple-name"].(proofing.SimplePropertyBlueprint)
			Expect(ok).To(BeTrue())
			Expect(simplePB.Name).To(Equal("some-simple-name"))

			Expect(propertyBlueprints).To(HaveKey(".properties.some-selector-name"))
			selectorPB, ok := propertyBlueprints[".properties.some-selector-name"].(proofing.SelectorPropertyBlueprint)
			Expect(ok).To(BeTrue())
			Expect(selectorPB.Name).To(Equal("some-selector-name"))

			Expect(propertyBlueprints).To(HaveKey(".properties.some-selector-name.some-option-template-name.some-nested-simple-name"))
			nestedSimplePB, ok :=
				propertyBlueprints[".properties.some-selector-name.some-option-template-name.some-nested-simple-name"].(proofing.SimplePropertyBlueprint)
			Expect(ok).To(BeTrue())
			Expect(nestedSimplePB.Name).To(Equal("some-nested-simple-name"))

			Expect(propertyBlueprints).To(HaveKey(".properties.some-collection-name"))
			collectionPB, ok := propertyBlueprints[".properties.some-collection-name"].(proofing.CollectionPropertyBlueprint)
			Expect(ok).To(BeTrue())
			Expect(collectionPB.Name).To(Equal("some-collection-name"))

			Expect(propertyBlueprints).To(HaveKey(".properties.some-collection-name.some-nested-simple-name"))
			nestedSimplePB, ok =
				propertyBlueprints[".properties.some-collection-name.some-nested-simple-name"].(proofing.SimplePropertyBlueprint)
			Expect(ok).To(BeTrue())
			Expect(nestedSimplePB.Name).To(Equal("some-nested-simple-name"))

			Expect(propertyBlueprints).To(HaveKey(".some-job-type-name.some-name"))
			jobTypePB, ok := propertyBlueprints[".some-job-type-name.some-name"].(proofing.SimplePropertyBlueprint)
			Expect(ok).To(BeTrue())
			Expect(jobTypePB.Name).To(Equal("some-name"))
		})
	})
})
