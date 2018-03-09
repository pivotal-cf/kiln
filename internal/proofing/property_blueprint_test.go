package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PropertyBlueprint", func() {
	var (
		simplePropertyBlueprint     proofing.SimplePropertyBlueprint
		selectorPropertyBlueprint   proofing.SelectorPropertyBlueprint
		collectionPropertyBlueprint proofing.CollectionPropertyBlueprint
	)

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/property_blueprints.yml")
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		simplePropertyBlueprint, ok = productTemplate.PropertyBlueprints[0].(proofing.SimplePropertyBlueprint)
		Expect(ok).To(BeTrue())

		selectorPropertyBlueprint, ok = productTemplate.PropertyBlueprints[1].(proofing.SelectorPropertyBlueprint)
		Expect(ok).To(BeTrue())

		collectionPropertyBlueprint, ok = productTemplate.PropertyBlueprints[2].(proofing.CollectionPropertyBlueprint)
		Expect(ok).To(BeTrue())
	})

	It("parses the different types of property blueprints", func() {
		Expect(simplePropertyBlueprint.Type).To(Equal("some-type"))
		Expect(selectorPropertyBlueprint.Type).To(Equal("selector"))
		Expect(collectionPropertyBlueprint.Type).To(Equal("collection"))
	})
})
