package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceDefinitions", func() {
	var resourceDefinition proofing.ResourceDefinition

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		resourceDefinition = productTemplate.JobTypes[0].ResourceDefinitions[0]
	})

	It("parses their structure", func() {
		Expect(resourceDefinition.Configurable).To(BeTrue())
		Expect(resourceDefinition.Constraints).To(Equal("some-constraints"))
		Expect(resourceDefinition.Default).To(Equal(1))
		Expect(resourceDefinition.Label).To(Equal("some-label"))
		Expect(resourceDefinition.Name).To(Equal("some-name"))
		Expect(resourceDefinition.Type).To(Equal("some-type"))
	})
})
