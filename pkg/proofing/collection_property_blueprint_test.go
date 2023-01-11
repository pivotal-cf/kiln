package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("CollectionPropertyBlueprint", func() {
	var collectionPropertyBlueprint proofing.CollectionPropertyBlueprint

	BeforeEach(func() {
		f, err := os.Open("fixtures/property_blueprints.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		var ok bool
		collectionPropertyBlueprint, ok = productTemplate.PropertyBlueprints[2].(proofing.CollectionPropertyBlueprint)
		Expect(ok).To(BeTrue())
	})

	It("parses their structure", func() {
		Expect(collectionPropertyBlueprint.Name).To(Equal("some-collection-name"))
		Expect(collectionPropertyBlueprint.Type).To(Equal("collection"))
		Expect(collectionPropertyBlueprint.Default).To(Equal("some-default"))
		Expect(collectionPropertyBlueprint.Constraints).To(Equal("some-constraints"))
		Expect(collectionPropertyBlueprint.Configurable).To(BeTrue())
		Expect(collectionPropertyBlueprint.Optional).To(BeTrue())
		Expect(collectionPropertyBlueprint.FreezeOnDeploy).To(BeFalse())
		Expect(collectionPropertyBlueprint.Unique).To(BeFalse())
		Expect(collectionPropertyBlueprint.ResourceDefinitions).To(HaveLen(1))
	})
})
