package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FormType", func() {
	var formType proofing.FormType

	BeforeEach(func() {
		metadata, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		formType = metadata.FormTypes[0]
	})

	It("parses their structure", func() {
		Expect(formType.Description).To(Equal("some-description"))
		Expect(formType.Label).To(Equal("some-label"))
		Expect(formType.Markdown).To(Equal("some-markdown"))
		Expect(formType.Name).To(Equal("some-name"))

		Expect(formType.PropertyInputs).To(HaveLen(1))
		Expect(formType.Verifiers).To(HaveLen(1))
	})

	Context("property_inputs", func() {
		var propertyInput proofing.FormTypePropertyInput

		BeforeEach(func() {
			propertyInput = formType.PropertyInputs[0]
		})

		It("parses their structure", func() {
			Expect(propertyInput.Description).To(Equal("some-description"))
			Expect(propertyInput.Label).To(Equal("some-label"))
			Expect(propertyInput.Placeholder).To(Equal("some-placeholder"))
			Expect(propertyInput.Reference).To(Equal("some-reference"))

			Expect(propertyInput.PropertyInputs).To(HaveLen(1))
		})

		Context("property_inputs", func() {
			It("parses their structure", func() {
				internalPropertyInput := propertyInput.PropertyInputs[0]

				Expect(internalPropertyInput.Description).To(Equal("some-description"))
				Expect(internalPropertyInput.Label).To(Equal("some-label"))
				Expect(internalPropertyInput.Reference).To(Equal("some-reference"))
			})
		})

		Context("selector_property_inputs", func() {
			var selectorPropertyInput proofing.FormTypePropertyInputSelectorPropertyInput

			BeforeEach(func() {
				selectorPropertyInput = propertyInput.SelectorPropertyInputs[0]
			})

			It("parses their structure", func() {
				Expect(selectorPropertyInput.Description).To(Equal("some-description"))
				Expect(selectorPropertyInput.Label).To(Equal("some-label"))
				Expect(selectorPropertyInput.Reference).To(Equal("some-reference"))

				Expect(selectorPropertyInput.PropertyInputs).To(HaveLen(1))
			})

			Context("property_inputs", func() {
				It("parses their structure", func() {
					internalPropertyInput := selectorPropertyInput.PropertyInputs[0]

					Expect(internalPropertyInput.Description).To(Equal("some-description"))
					Expect(internalPropertyInput.Label).To(Equal("some-label"))
					Expect(internalPropertyInput.Placeholder).To(Equal("some-placeholder"))
					Expect(internalPropertyInput.Reference).To(Equal("some-reference"))
				})
			})
		})
	})
})
