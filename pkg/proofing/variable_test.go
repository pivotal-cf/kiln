package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("Variable", func() {
	var variable proofing.Variable

	BeforeEach(func() {
		f, err := os.Open("testdata/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		variable = productTemplate.Variables[0]
	})

	It("parses their structure", func() {
		Expect(variable.Name).To(Equal("some-name"))
		Expect(variable.Options).To(Equal("some-options"))
		Expect(variable.Type).To(Equal("some-type"))
	})
})
