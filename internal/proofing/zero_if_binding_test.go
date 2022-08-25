package proofing_test

import (
	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ZeroIf", func() {
	var zeroIfBinding proofing2.ZeroIfBinding

	BeforeEach(func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		zeroIfBinding = productTemplate.JobTypes[0].InstanceDefinition.ZeroIf
	})

	It("parses their structure", func() {
		Expect(zeroIfBinding.PropertyReference).To(Equal("some-property-reference"))
		Expect(zeroIfBinding.PropertyValue).To(Equal("some-property-value"))
	})
})
