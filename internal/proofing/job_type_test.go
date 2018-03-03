package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobType", func() {
	var jobType proofing.JobType

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		jobType = productTemplate.JobTypes[0]
	})

	It("parses their structure", func() {
		Expect(jobType.Description).To(Equal("some-description"))
		Expect(jobType.DynamicIP).To(Equal(2))
		Expect(jobType.Label).To(Equal("some-label"))
		Expect(jobType.MaxInFlight).To(Equal("some-max-in-flight"))
		Expect(jobType.Name).To(Equal("some-name"))
		Expect(jobType.ResourceLabel).To(Equal("some-resource-label"))
		Expect(jobType.Serial).To(BeTrue())
		Expect(jobType.SingleAZOnly).To(BeTrue())
		Expect(jobType.StaticIP).To(Equal(1))

		Expect(jobType.InstanceDefinition).To(BeAssignableToTypeOf(proofing.InstanceDefinition{}))
		Expect(jobType.PropertyBlueprints).To(HaveLen(1))
		Expect(jobType.ResourceDefinitions).To(HaveLen(1))
		Expect(jobType.Templates).To(HaveLen(1))
	})

	Context("property_blueprints", func() {
		It("parses their structure", func() {
			propertyBlueprint := jobType.PropertyBlueprints[0]

			Expect(propertyBlueprint.Configurable).To(BeTrue())
			Expect(propertyBlueprint.Constraints).To(Equal("some-constraints"))
			Expect(propertyBlueprint.Default).To(Equal("some-default"))
			Expect(propertyBlueprint.Label).To(Equal("some-label"))
			Expect(propertyBlueprint.Name).To(Equal("some-name"))
			Expect(propertyBlueprint.Optional).To(BeTrue())
			Expect(propertyBlueprint.Type).To(Equal("some-type"))
		})
	})
})
