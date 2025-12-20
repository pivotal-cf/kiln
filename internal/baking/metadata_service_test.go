package baking_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/pivotal-cf/kiln/internal/baking"
)

var _ = Describe("MetadataService", func() {
	Describe("Read", func() {
		var (
			path    string
			service MetadataService
		)

		BeforeEach(func() {
			file, err := os.CreateTemp("", "metadata")
			Expect(err).NotTo(HaveOccurred())

			path = file.Name()

			_, err = file.WriteString("some-metadata")
			Expect(err).NotTo(HaveOccurred())

			Expect(file.Close()).To(Succeed())

			service = NewMetadataService()
		})

		AfterEach(func() {
			Expect(os.Remove(path)).To(Succeed())
		})

		It("reads the file", func() {
			contents, err := service.Read(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal([]byte("some-metadata")))
		})

		Context("failure cases", func() {
			Context("when the file does not exist", func() {
				It("returns an error", func() {
					_, err := service.Read("missing-metadata")
					Expect(err).To(MatchError(ContainSubstring("open missing-metadata: no such file or directory")))
				})
			})
		})
	})
})
