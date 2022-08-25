package proofing_test

import (
	"errors"
	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PropertyInputs", func() {
	var formType proofing2.FormType

	BeforeEach(func() {
		f, err := os.Open("fixtures/form_types.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		formType = productTemplate.FormTypes[0]
	})

	It("parses the different types", func() {
		Expect(formType.PropertyInputs[0]).To(BeAssignableToTypeOf(proofing2.SimplePropertyInput{}))
		Expect(formType.PropertyInputs[1]).To(BeAssignableToTypeOf(proofing2.CollectionPropertyInput{}))
		Expect(formType.PropertyInputs[2]).To(BeAssignableToTypeOf(proofing2.SelectorPropertyInput{}))
	})

	Context("failure cases", func() {
		Context("when the YAML cannot be unmarshalled", func() {
			It("returns an error", func() {
				propertyInputs := proofing2.PropertyInputs([]proofing2.PropertyInput{})

				err := propertyInputs.UnmarshalYAML(func(v interface{}) error {
					return errors.New("unmarshal failed")
				})

				Expect(err).To(MatchError("unmarshal failed"))
			})
		})
	})
})
