package proofing_test

import (
	"os"

	proofing2 "github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuntimeConfigTemplate", func() {
	var runtimeConfigTemplate proofing2.RuntimeConfigTemplate

	BeforeEach(func() {
		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing2.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		runtimeConfigTemplate = productTemplate.RuntimeConfigs[0]
	})

	It("parses their structure", func() {
		Expect(runtimeConfigTemplate.Name).To(Equal("some-name"))
		Expect(runtimeConfigTemplate.RuntimeConfig).To(Equal("some-runtime-config"))
	})
})
