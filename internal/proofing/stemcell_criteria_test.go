package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StemcellCriteria", func() {
	var stemcellCriteria proofing.StemcellCriteria

	BeforeEach(func() {
		metadata, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		stemcellCriteria = metadata.StemcellCriteria
	})

	It("parses its structure", func() {
		Expect(stemcellCriteria.OS).To(Equal("some-os"))
		Expect(stemcellCriteria.Version).To(Equal("some-version"))
	})
})
