package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("ZeroIf", func() {
	var zeroIfBinding proofing.ZeroIfBinding

	BeforeEach(func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer func() { _ = f.Close() }()
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		zeroIfBinding = productTemplate.JobTypes[0].InstanceDefinition.ZeroIf
	})

	It("parses their structure", func() {
		Expect(zeroIfBinding.PropertyReference).To(Equal("some-property-reference"))
		Expect(zeroIfBinding.PropertyValue).To(Equal("some-property-value"))
	})
})
