package proofing_test

import (
	"os"

	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VerifierBlueprint", func() {
	var verifierBlueprint proofing2.VerifierBlueprint

	BeforeEach(func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		verifierBlueprint = productTemplate.FormTypes[0].Verifiers[0]
	})

	It("parses their structure", func() {
		Expect(verifierBlueprint.Name).To(Equal("some-name"))
		Expect(verifierBlueprint.Properties).To(Equal("some-properties"))
	})
})
