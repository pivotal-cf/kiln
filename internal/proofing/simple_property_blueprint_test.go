package proofing_test

import (
	"os"

	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SimplePropertyBlueprint", func() {
	var simplePropertyBlueprint proofing2.SimplePropertyBlueprint

	BeforeEach(func() {
		f, err := os.Open("fixtures/property_blueprints.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		simplePropertyBlueprint, ok = productTemplate.PropertyBlueprints[0].(proofing2.SimplePropertyBlueprint)
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

	Describe("Normalize", func() {
		It("returns a list of normalized property blueprints", func() {
			normalized := simplePropertyBlueprint.Normalize("some-prefix")

			Expect(normalized).To(ConsistOf([]proofing2.NormalizedPropertyBlueprint{
				{
					Property:     "some-prefix.some-simple-name",
					Configurable: true,
					Default:      "some-default",
					Required:     false,
					Type:         "some-type",
				},
			}))
		})

		Context("when the property blueprint is not optional", func() {
			It("marks the property blueprint as required", func() {
				simplePropertyBlueprint.Optional = false

				normalized := simplePropertyBlueprint.Normalize("some-prefix")
				Expect(normalized[0].Required).To(BeTrue())
			})
		})
	})

	Context("options", func() {
		It("parses their structure", func() {
			option := simplePropertyBlueprint.Options[0]

			Expect(option.Label).To(Equal("some-label"))
			Expect(option.Name).To(Equal("some-name"))
		})
	})
})
