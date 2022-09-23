package proofing_test

import (
	"errors"
	"os"

	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PropertyBlueprints", func() {
	var productTemplate proofing2.ProductTemplate

	BeforeEach(func() {
		f, err := os.Open("fixtures/property_blueprints.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err = proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())
	})

	It("parses the different types", func() {
		Expect(productTemplate.PropertyBlueprints[0]).To(BeAssignableToTypeOf(proofing2.SimplePropertyBlueprint{}))
		Expect(productTemplate.PropertyBlueprints[1]).To(BeAssignableToTypeOf(proofing2.SelectorPropertyBlueprint{}))
		Expect(productTemplate.PropertyBlueprints[2]).To(BeAssignableToTypeOf(proofing2.CollectionPropertyBlueprint{}))
	})

	Context("failure cases", func() {
		Context("when the YAML cannot be unmarshalled", func() {
			It("returns an error", func() {
				propertyBlueprints := proofing2.PropertyBlueprints([]proofing2.PropertyBlueprint{})

				err := propertyBlueprints.UnmarshalYAML(func(v interface{}) error {
					return errors.New("unmarshal failed")
				})
				Expect(err).To(MatchError("unmarshal failed"))
			})
		})
	})
})
