package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("StemcellCriteria", func() {
	var stemcellCriteria proofing.StemcellCriteria

	BeforeEach(func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		stemcellCriteria = productTemplate.StemcellCriteria
	})

	It("parses its structure", func() {
		Expect(stemcellCriteria.OS).To(Equal("some-os"))
		Expect(stemcellCriteria.Version).To(Equal("some-version"))
		Expect(stemcellCriteria.EnablePatchSecurityUpdates).To(BeTrue())
	})
})
