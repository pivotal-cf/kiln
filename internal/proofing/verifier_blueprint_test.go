package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VerifierBlueprint", func() {
	var verifierBlueprint proofing.VerifierBlueprint

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		verifierBlueprint = productTemplate.FormTypes[0].Verifiers[0]
	})

	It("parses their structure", func() {
		Expect(verifierBlueprint.Name).To(Equal("some-name"))
		Expect(verifierBlueprint.Properties).To(Equal("some-properties"))
	})
})
