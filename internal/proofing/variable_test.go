package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Variable", func() {
	var variable proofing.Variable

	BeforeEach(func() {
		metadata, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		variable = metadata.Variables[0]
	})

	It("parses their structure", func() {
		Expect(variable.Name).To(Equal("some-name"))
		Expect(variable.Options).To(Equal("some-options"))
		Expect(variable.Type).To(Equal("some-type"))
	})
})
