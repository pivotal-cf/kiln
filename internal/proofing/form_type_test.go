package proofing_test

import (
	"os"

	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FormType", func() {
	var formType proofing2.FormType

	BeforeEach(func() {
		f, err := os.Open("fixtures/form_types.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		formType = productTemplate.FormTypes[0]
	})

	It("parses their structure", func() {
		Expect(formType.Description).To(Equal("some-description"))
		Expect(formType.Label).To(Equal("some-label"))
		Expect(formType.Markdown).To(Equal("some-markdown"))
		Expect(formType.Name).To(Equal("some-name"))

		Expect(formType.PropertyInputs).To(HaveLen(3))
		Expect(formType.Verifiers).To(HaveLen(1))
	})
})
