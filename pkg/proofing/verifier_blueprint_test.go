package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("VerifierBlueprint", func() {
	var verifierBlueprint proofing.VerifierBlueprint

	BeforeEach(func() {
		f, err := os.Open("testdata/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		verifierBlueprint = productTemplate.FormTypes[0].Verifiers[0]
	})

	It("parses their structure", func() {
		Expect(verifierBlueprint.Name).To(Equal("some-name"))
		Expect(verifierBlueprint.Properties).To(Equal("some-properties"))
	})
})
