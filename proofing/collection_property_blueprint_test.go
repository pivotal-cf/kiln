package proofing_test

import (
	"github.com/pivotal-cf/kiln/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CollectionPropertyBlueprint", func() {
	var collectionPropertyBlueprint proofing.CollectionPropertyBlueprint

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/property_blueprints.yml")
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		collectionPropertyBlueprint, ok = productTemplate.PropertyBlueprints[2].(proofing.CollectionPropertyBlueprint)
		Expect(ok).To(BeTrue())
	})

	It("parses their structure", func() {
		Expect(collectionPropertyBlueprint.Name).To(Equal("some-collection-name"))
		Expect(collectionPropertyBlueprint.Type).To(Equal("collection"))
		Expect(collectionPropertyBlueprint.Default).To(Equal("some-default"))
		Expect(collectionPropertyBlueprint.Constraints).To(Equal("some-constraints"))
		Expect(collectionPropertyBlueprint.Options).To(HaveLen(1))
		Expect(collectionPropertyBlueprint.Configurable).To(BeTrue())
		Expect(collectionPropertyBlueprint.Optional).To(BeTrue())
		Expect(collectionPropertyBlueprint.FreezeOnDeploy).To(BeFalse())
		Expect(collectionPropertyBlueprint.Unique).To(BeFalse())
		Expect(collectionPropertyBlueprint.ResourceDefinitions).To(HaveLen(1))
	})

	Context("options", func() {
		It("parses their structure", func() {
			option := collectionPropertyBlueprint.Options[0]

			Expect(option.Label).To(Equal("some-label"))
			Expect(option.Name).To(Equal("some-name"))
		})
	})
})
