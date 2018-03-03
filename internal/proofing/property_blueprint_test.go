package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PropertyBlueprint", func() {
	var propertyBlueprint proofing.PropertyBlueprint

	BeforeEach(func() {
		metadata, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		propertyBlueprint = metadata.PropertyBlueprints[0]
	})

	It("parses their structure", func() {
		Expect(propertyBlueprint.Configurable).To(BeTrue())
		Expect(propertyBlueprint.Default).To(Equal("some-default"))
		Expect(propertyBlueprint.Name).To(Equal("some-name"))
		Expect(propertyBlueprint.Optional).To(BeTrue())
		Expect(propertyBlueprint.Type).To(Equal("some-type"))

		Expect(propertyBlueprint.NamedManifests).To(HaveLen(1))
		Expect(propertyBlueprint.OptionTemplates).To(HaveLen(1))
		Expect(propertyBlueprint.Options).To(HaveLen(1))
		Expect(propertyBlueprint.PropertyBlueprints).To(HaveLen(1))
	})

	Context("named_manifests", func() {
		It("parses their structure", func() {
			namedManifest := propertyBlueprint.NamedManifests[0]

			Expect(namedManifest.Manifest).To(Equal("some-manifest"))
			Expect(namedManifest.Name).To(Equal("some-name"))
		})
	})

	Context("option_templates", func() {
		var optionTemplate proofing.PropertyBlueprintOptionTemplate

		BeforeEach(func() {
			optionTemplate = propertyBlueprint.OptionTemplates[0]
		})

		It("parses their structure", func() {
			Expect(optionTemplate.Name).To(Equal("some-name"))
			Expect(optionTemplate.SelectValue).To(Equal("some-select-value"))

			Expect(optionTemplate.NamedManifests).To(HaveLen(1))
			Expect(optionTemplate.PropertyBlueprints).To(HaveLen(1))
		})

		Context("named_manifests", func() {
			It("parses their structure", func() {
				namedManifest := optionTemplate.NamedManifests[0]

				Expect(namedManifest.Manifest).To(Equal("some-manifest"))
				Expect(namedManifest.Name).To(Equal("some-name"))
			})
		})

		Context("property_blueprints", func() {
			var internalPropertyBlueprint proofing.PropertyBlueprintOptionTemplatePropertyBlueprint

			BeforeEach(func() {
				internalPropertyBlueprint = optionTemplate.PropertyBlueprints[0]
			})

			It("parses their structure", func() {
				Expect(internalPropertyBlueprint.Configurable).To(BeTrue())
				Expect(internalPropertyBlueprint.Constraints).To(Equal("some-constraints"))
				Expect(internalPropertyBlueprint.Default).To(Equal(1))
				Expect(internalPropertyBlueprint.Name).To(Equal("some-name"))
				Expect(internalPropertyBlueprint.Optional).To(BeTrue())
				Expect(internalPropertyBlueprint.Placeholder).To(Equal("some-placeholder"))
				Expect(internalPropertyBlueprint.Type).To(Equal("some-type"))

				Expect(internalPropertyBlueprint.Options).To(HaveLen(1))
			})

			Context("options", func() {
				It("parses their structure", func() {
					option := internalPropertyBlueprint.Options[0]

					Expect(option.Label).To(Equal("some-label"))
					Expect(option.Name).To(Equal("some-name"))
				})
			})
		})
	})

	Context("options", func() {
		It("parses their structure", func() {
			option := propertyBlueprint.Options[0]

			Expect(option.Label).To(Equal("some-label"))
			Expect(option.Name).To(Equal("some-name"))
		})
	})

	Context("property_blueprints", func() {
		It("parses their structure", func() {
			internalPropertyBlueprint := propertyBlueprint.PropertyBlueprints[0]

			Expect(internalPropertyBlueprint.Configurable).To(BeTrue())
			Expect(internalPropertyBlueprint.Default).To(Equal("some-default"))
			Expect(internalPropertyBlueprint.Name).To(Equal("some-name"))
			Expect(internalPropertyBlueprint.Type).To(Equal("some-type"))
		})
	})
})
