package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("InstanceDefinition", func() {
	var instanceDefinition proofing.InstanceDefinition

	BeforeEach(func() {
		f, err := os.Open("testdata/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		instanceDefinition = productTemplate.JobTypes[0].InstanceDefinition
	})

	It("parses their structure", func() {
		Expect(instanceDefinition.Configurable).To(BeTrue())
		Expect(instanceDefinition.Default).To(Equal(2))
		Expect(instanceDefinition.ZeroIf.PropertyReference).To(Equal("some-property-reference"))
		Expect(instanceDefinition.Constraints).To(Equal(&proofing.IntegerConstraints{Min: ptr(1), Max: ptr(5)}))
	})
})
