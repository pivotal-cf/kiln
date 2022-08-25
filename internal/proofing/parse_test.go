package proofing_test

import (
	"errors"
	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parse", func() {
	It("can parse a metadata file", func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(productTemplate).To(BeAssignableToTypeOf(proofing2.ProductTemplate{}))
	})

	Context("failure cases", func() {
		Context("when the metadata file cannot be read", func() {
			It("returns an error", func() {
				_, err := proofing2.Parse(erroringReader{})
				Expect(err).To(MatchError("failed to read"))
			})
		})

		Context("when the metadata contents cannot be unmarshalled", func() {
			It("returns an error", func() {
				f, err := os.Open("fixtures/malformed.yml")
				defer closeAndIgnoreError(f)
				Expect(err).NotTo(HaveOccurred())

				_, err = proofing2.Parse(f)
				Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
			})
		})
	})
})

type erroringReader struct{}

func (r erroringReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("failed to read")
}
