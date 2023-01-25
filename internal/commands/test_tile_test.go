package commands

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test", func() {
	Context("manifest tests succeed", func() {
		It("Executes manifest tests and reports success", func() {
			Expect(true).To(Equal(false))
		})

		Context("manifest tests succeed", func() {
			It("Executes manifest tests and reports failure", func() {
				Expect(true).To(Equal(false))
			})
		})
	})
})
