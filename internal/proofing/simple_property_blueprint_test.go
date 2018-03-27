package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SimplePropertyBlueprint", func() {
	var simplePropertyBlueprint proofing.SimplePropertyBlueprint

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/property_blueprints.yml")
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		simplePropertyBlueprint, ok = productTemplate.PropertyBlueprints[0].(proofing.SimplePropertyBlueprint)
		Expect(ok).To(BeTrue())
	})

	It("parses their structure", func() {
		Expect(simplePropertyBlueprint.Name).To(Equal("some-simple-name"))
		Expect(simplePropertyBlueprint.Type).To(Equal("some-type"))
		Expect(simplePropertyBlueprint.Default).To(Equal("some-default"))
		Expect(simplePropertyBlueprint.Constraints).To(Equal("some-constraints"))
		Expect(simplePropertyBlueprint.Options).To(HaveLen(1))
		Expect(simplePropertyBlueprint.Configurable).To(BeTrue())
		Expect(simplePropertyBlueprint.Optional).To(BeTrue())
		Expect(simplePropertyBlueprint.FreezeOnDeploy).To(BeTrue())
		Expect(simplePropertyBlueprint.Unique).To(BeTrue())
		Expect(simplePropertyBlueprint.ResourceDefinitions).To(HaveLen(1))
	})

	Context("options", func() {
		It("parses their structure", func() {
			option := simplePropertyBlueprint.Options[0]

			Expect(option.Label).To(Equal("some-label"))
			Expect(option.Name).To(Equal("some-name"))
		})
	})
})
