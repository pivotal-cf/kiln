package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstallTimeVerifier", func() {
	var installTimeVerifier proofing.InstallTimeVerifier

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		installTimeVerifier = productTemplate.InstallTimeVerifiers[0]
	})

	It("parses their structure", func() {
		Expect(installTimeVerifier.Ignorable).To(BeTrue())
		Expect(installTimeVerifier.Name).To(Equal("some-name"))

		Expect(installTimeVerifier.Properties).To(Equal("some-properties"))
	})
})
