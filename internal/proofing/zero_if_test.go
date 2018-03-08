package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ZeroIf", func() {
	var zeroIf proofing.ZeroIf

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		zeroIf = productTemplate.JobTypes[0].InstanceDefinition.ZeroIf
	})

	It("parses their structure", func() {
		Expect(zeroIf.PropertyReference).To(Equal("some-property-reference"))
		Expect(zeroIf.PropertyValue).To(Equal("some-property-value"))
	})
})
