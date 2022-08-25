package proofing_test

import (
	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceDefinitions", func() {
	var resourceDefinition proofing2.ResourceDefinition

	BeforeEach(func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		resourceDefinition = productTemplate.JobTypes[0].ResourceDefinitions[0]
	})

	It("parses their structure", func() {
		Expect(resourceDefinition.Configurable).To(BeTrue())
		Expect(resourceDefinition.Constraints).To(Equal("some-constraints"))
		Expect(resourceDefinition.Default).To(Equal(1))
		Expect(resourceDefinition.Name).To(Equal("some-name"))
	})
})
