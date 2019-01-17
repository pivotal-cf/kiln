package commands_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("Fetch", func() {

	Describe("Execute", func() {
		It("fetches releases", func() {
			err := fetch.Execute()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
