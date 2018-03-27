package proofing_test

import (
	"github.com/pivotal-cf/kiln/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InstanceDefinition", func() {
	var instanceDefinition proofing.InstanceDefinition

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		instanceDefinition = productTemplate.JobTypes[0].InstanceDefinition
	})

	It("parses their structure", func() {
		Expect(instanceDefinition.Configurable).To(BeTrue())
		Expect(instanceDefinition.Default).To(Equal(2))
		Expect(instanceDefinition.ZeroIf.PropertyReference).To(Equal("some-property-reference"))
		Expect(instanceDefinition.Constraints).To(Equal("some-constraints"))
	})
})
