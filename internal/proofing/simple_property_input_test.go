package proofing_test

import (
	"os"

	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SimplePropertyInput", func() {
	var simplePropertyInput proofing2.SimplePropertyInput

	BeforeEach(func() {
		f, err := os.Open("fixtures/form_types.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		simplePropertyInput, ok = productTemplate.FormTypes[0].PropertyInputs[0].(proofing2.SimplePropertyInput)
		Expect(ok).To(BeTrue())
	})

	It("parses their structure", func() {
		Expect(simplePropertyInput.Description).To(Equal("some-description"))
		Expect(simplePropertyInput.Label).To(Equal("some-label"))
		Expect(simplePropertyInput.Placeholder).To(Equal("some-placeholder"))
		Expect(simplePropertyInput.Reference).To(Equal("some-reference"))
	})
})
