package commands

import (
	"bytes"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Test", func() {
	Context("manifest tests succeed", func() {
		FIt("logs test output", func() {
			var testOutput bytes.Buffer
			logger := log.New(&testOutput, "", 0)
			testTile := NewTestTile(logger)
			err := testTile.Execute([]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(testOutput.String()).To(ContainSubstring("ok"))
		})

		// It("captures success code", func() {

		// 	Expect(successCode).To(Equal(0))
		// })

		// Context("manifest tests succeed", func() {
		// 	It("Executes manifest tests and reports failure", func() {
		// 		Expect(true).To(Equal(false))
		// 	})
		// })
	})
})
