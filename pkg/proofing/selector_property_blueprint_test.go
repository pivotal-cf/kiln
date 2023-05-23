package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("SelectorPropertyBlueprint", func() {
	var selectorPropertyBlueprint *proofing.SelectorPropertyBlueprint

	BeforeEach(func() {
		f, err := os.Open("testdata/property_blueprints.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		selectorPropertyBlueprint, ok = productTemplate.PropertyBlueprints[1].(*proofing.SelectorPropertyBlueprint)
		Expect(ok).To(BeTrue())
	})

	It("parses their structure", func() {
		Expect(selectorPropertyBlueprint.Configurable).To(BeTrue())
		Expect(selectorPropertyBlueprint.Constraints).To(Equal("some-constraints"))
		Expect(selectorPropertyBlueprint.Default).To(Equal("some-default"))
		Expect(selectorPropertyBlueprint.FreezeOnDeploy).To(BeTrue())
		Expect(selectorPropertyBlueprint.Name).To(Equal("some-selector-name"))
		Expect(selectorPropertyBlueprint.Optional).To(BeTrue())
		Expect(selectorPropertyBlueprint.Type).To(Equal("selector"))
		Expect(selectorPropertyBlueprint.Unique).To(BeTrue())
		Expect(selectorPropertyBlueprint.ResourceDefinitions).To(HaveLen(1))
		Expect(selectorPropertyBlueprint.OptionTemplates).To(HaveLen(1))
	})
})
