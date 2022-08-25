package proofing_test

import (
	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StemcellCriteria", func() {
	var stemcellCriteria proofing2.StemcellCriteria

	BeforeEach(func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		stemcellCriteria = productTemplate.StemcellCriteria
	})

	It("parses its structure", func() {
		Expect(stemcellCriteria.OS).To(Equal("some-os"))
		Expect(stemcellCriteria.Version).To(Equal("some-version"))
		Expect(stemcellCriteria.EnablePatchSecurityUpdates).To(BeTrue())
	})
})
