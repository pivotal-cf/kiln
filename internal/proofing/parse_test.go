package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parse", func() {
	It("can parse a metadata file", func() {
		metadata, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())
		Expect(metadata).To(BeAssignableToTypeOf(proofing.ProductTemplate{}))
	})

	Context("failure cases", func() {
		Context("when the metadata file cannot be found", func() {
			It("returns an error", func() {
				_, err := proofing.Parse("missing-file.yml")
				Expect(err).To(MatchError(ContainSubstring("missing-file.yml: no such file or directory")))
			})
		})

		Context("when the metadata contents cannot be unmarshalled", func() {
			It("returns an error", func() {
				_, err := proofing.Parse("fixtures/malformed.yml")
				Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
			})
		})
	})
})
