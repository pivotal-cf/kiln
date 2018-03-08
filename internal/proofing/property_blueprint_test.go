package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PropertyBlueprint", func() {
	var (
		simplePropertyBlueprint     proofing.SimplePropertyBlueprint
		selectorPropertyBlueprint   proofing.SelectorPropertyBlueprint
		collectionPropertyBlueprint proofing.CollectionPropertyBlueprint
	)

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		simplePropertyBlueprint, ok = productTemplate.PropertyBlueprints[0].(proofing.SimplePropertyBlueprint)
		Expect(ok).To(BeTrue())

		selectorPropertyBlueprint, ok = productTemplate.PropertyBlueprints[1].(proofing.SelectorPropertyBlueprint)
		Expect(ok).To(BeTrue())

		collectionPropertyBlueprint, ok = productTemplate.PropertyBlueprints[2].(proofing.CollectionPropertyBlueprint)
		Expect(ok).To(BeTrue())
	})

	It("parses the different types of property blueprints", func() {
		Expect(simplePropertyBlueprint.Type).To(Equal("some-type"))
		Expect(selectorPropertyBlueprint.Type).To(Equal("selector"))
		Expect(collectionPropertyBlueprint.Type).To(Equal("collection"))
	})

	It("parses their structure", func() {
		Expect(simplePropertyBlueprint.Configurable).To(BeTrue())
		Expect(simplePropertyBlueprint.Default).To(Equal("some-default"))
		Expect(simplePropertyBlueprint.Name).To(Equal("some-name"))
		Expect(simplePropertyBlueprint.Optional).To(BeTrue())
		Expect(simplePropertyBlueprint.Type).To(Equal("some-type"))

		Expect(simplePropertyBlueprint.NamedManifests).To(HaveLen(1))
		Expect(simplePropertyBlueprint.OptionTemplates).To(HaveLen(1))
		Expect(simplePropertyBlueprint.Options).To(HaveLen(1))
		Expect(simplePropertyBlueprint.PropertyBlueprints).To(HaveLen(1))
	})

	Context("named_manifests", func() {
		It("parses their structure", func() {
			namedManifest := simplePropertyBlueprint.NamedManifests[0]

			Expect(namedManifest.Manifest).To(Equal("some-manifest"))
			Expect(namedManifest.Name).To(Equal("some-name"))
		})
	})

	Context("option_templates", func() {
		var optionTemplate proofing.PropertyBlueprintOptionTemplate

		BeforeEach(func() {
			optionTemplate = simplePropertyBlueprint.OptionTemplates[0]
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
			option := simplePropertyBlueprint.Options[0]

			Expect(option.Label).To(Equal("some-label"))
			Expect(option.Name).To(Equal("some-name"))
		})
	})

	Context("property_blueprints", func() {
		It("parses their structure", func() {
			internalPropertyBlueprint := simplePropertyBlueprint.PropertyBlueprints[0]

			Expect(internalPropertyBlueprint.Configurable).To(BeTrue())
			Expect(internalPropertyBlueprint.Default).To(Equal("some-default"))
			Expect(internalPropertyBlueprint.Name).To(Equal("some-name"))
			Expect(internalPropertyBlueprint.Type).To(Equal("some-type"))
		})
	})
})
