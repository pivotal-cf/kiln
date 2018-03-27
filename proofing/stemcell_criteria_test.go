package proofing_test

import (
	"github.com/pivotal-cf/kiln/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StemcellCriteria", func() {
	var stemcellCriteria proofing.StemcellCriteria

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		stemcellCriteria = productTemplate.StemcellCriteria
	})

	It("parses its structure", func() {
		Expect(stemcellCriteria.OS).To(Equal("some-os"))
		Expect(stemcellCriteria.Version).To(Equal("some-version"))
		Expect(stemcellCriteria.EnablePatchSecurityUpdates).To(BeTrue())
	})
})
