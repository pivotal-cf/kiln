package proofing_test

import (
	"os"

	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SelectorPropertyInput", func() {
	var selectorPropertyInput proofing2.SelectorPropertyInput

	BeforeEach(func() {
		f, err := os.Open("fixtures/form_types.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		selectorPropertyInput, ok = productTemplate.FormTypes[0].PropertyInputs[2].(proofing2.SelectorPropertyInput)
		Expect(ok).To(BeTrue())
	})

	It("parses their structure", func() {
		Expect(selectorPropertyInput.Description).To(Equal("some-description"))
		Expect(selectorPropertyInput.Label).To(Equal("some-label"))
		Expect(selectorPropertyInput.Placeholder).To(Equal("some-placeholder"))
		Expect(selectorPropertyInput.Reference).To(Equal("some-reference"))

		Expect(selectorPropertyInput.SelectorPropertyInputs).To(HaveLen(1))
	})

	Context("selector_property_inputs", func() {
		var selectorOptionPropertyInput proofing2.SelectorOptionPropertyInput

		BeforeEach(func() {
			selectorOptionPropertyInput = selectorPropertyInput.SelectorPropertyInputs[0]
		})

		It("parses their structure", func() {
			Expect(selectorOptionPropertyInput.Label).To(Equal("some-label"))
			Expect(selectorOptionPropertyInput.Reference).To(Equal("some-reference"))

			Expect(selectorOptionPropertyInput.PropertyInputs).To(HaveLen(1))
		})

		Context("property_inputs", func() {
			It("parses their structure", func() {
				propertyInput := selectorOptionPropertyInput.PropertyInputs[0]

				Expect(propertyInput.Description).To(Equal("some-description"))
				Expect(propertyInput.Label).To(Equal("some-label"))
				Expect(propertyInput.Placeholder).To(Equal("some-placeholder"))
				Expect(propertyInput.Reference).To(Equal("some-reference"))
			})
		})
	})
})
