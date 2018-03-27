package proofing_test

import (
	"github.com/pivotal-cf/kiln/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ZeroIf", func() {
	var zeroIfBinding proofing.ZeroIfBinding

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		zeroIfBinding = productTemplate.JobTypes[0].InstanceDefinition.ZeroIf
	})

	It("parses their structure", func() {
		Expect(zeroIfBinding.PropertyReference).To(Equal("some-property-reference"))
		Expect(zeroIfBinding.PropertyValue).To(Equal("some-property-value"))
	})
})
