package proofing_test

import (
	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstallTimeVerifier", func() {
	var installTimeVerifier proofing2.InstallTimeVerifier

	BeforeEach(func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		installTimeVerifier = productTemplate.InstallTimeVerifiers[0]
	})

	It("parses their structure", func() {
		Expect(installTimeVerifier.Ignorable).To(BeTrue())
		Expect(installTimeVerifier.Name).To(Equal("some-name"))

		Expect(installTimeVerifier.Properties).To(Equal("some-properties"))
	})
})
