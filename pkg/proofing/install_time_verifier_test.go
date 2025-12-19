package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("InstallTimeVerifier", func() {
	var installTimeVerifier proofing.InstallTimeVerifier

	BeforeEach(func() {
		f, err := os.Open("testdata/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		installTimeVerifier = productTemplate.InstallTimeVerifiers[0]
	})

	It("parses their structure", func() {
		Expect(installTimeVerifier.Ignorable).To(BeTrue())
		Expect(installTimeVerifier.Name).To(Equal("some-name"))

		Expect(installTimeVerifier.Properties).To(Equal("some-properties"))
	})
})
