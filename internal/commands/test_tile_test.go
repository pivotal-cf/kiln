package commands

import (
	"bytes"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test", func() {
	Context("manifest tests succeed", func() {
		It("logs test output", func() {
			var testOutput bytes.Buffer
			logger := log.New(&testOutput, "", 0)
			testTile := NewTestTile(logger)
			testTile.Execute([]string{})
			Expect(testOutput).To(ContainSubstring("success test output"))
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
