package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PropertyBlueprints", func() {
	var productTemplate proofing.ProductTemplate

	BeforeEach(func() {
		var err error
		productTemplate, err = proofing.Parse("fixtures/property_blueprints.yml")
		Expect(err).NotTo(HaveOccurred())
	})

	It("parses their structure", func() {
		Expect(productTemplate.PropertyBlueprints[0]).To(BeAssignableToTypeOf(proofing.SimplePropertyBlueprint{}))
		Expect(productTemplate.PropertyBlueprints[1]).To(BeAssignableToTypeOf(proofing.SelectorPropertyBlueprint{}))
		Expect(productTemplate.PropertyBlueprints[2]).To(BeAssignableToTypeOf(proofing.CollectionPropertyBlueprint{}))
	})
})
