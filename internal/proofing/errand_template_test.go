package proofing_test

import (
	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ErrandTemplate", func() {
	var errandTemplate proofing2.ErrandTemplate

	BeforeEach(func() {
		f, err := os.Open("fixtures/errands.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		errandTemplate = productTemplate.PostDeployErrands[0]
	})

	It("parses their structure", func() {
		Expect(errandTemplate.Colocated).To(BeTrue())
		Expect(errandTemplate.Description).To(Equal("some-description"))
		Expect(errandTemplate.Instances).To(HaveLen(1))
		Expect(errandTemplate.Label).To(Equal("some-label"))
		Expect(errandTemplate.Name).To(Equal("some-name"))
		Expect(errandTemplate.RunDefault).To(BeTrue())
	})
})
