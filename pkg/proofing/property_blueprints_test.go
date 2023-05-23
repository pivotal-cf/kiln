package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("PropertyBlueprints", func() {
	var productTemplate proofing.ProductTemplate

	BeforeEach(func() {
		f, err := os.Open("testdata/property_blueprints.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err = proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())
	})

	It("parses the different types", func() {
		Expect(productTemplate.PropertyBlueprints[0]).To(BeAssignableToTypeOf(&proofing.SimplePropertyBlueprint{}))
		Expect(productTemplate.PropertyBlueprints[1]).To(BeAssignableToTypeOf(&proofing.SelectorPropertyBlueprint{}))
		Expect(productTemplate.PropertyBlueprints[2]).To(BeAssignableToTypeOf(&proofing.CollectionPropertyBlueprint{}))
	})
})
