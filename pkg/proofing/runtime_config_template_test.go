package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("RuntimeConfigTemplate", func() {
	var runtimeConfigTemplate proofing.RuntimeConfigTemplate

	BeforeEach(func() {
		f, err := os.Open("testdata/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err := proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())

		runtimeConfigTemplate = productTemplate.RuntimeConfigs[0]
	})

	It("parses their structure", func() {
		Expect(runtimeConfigTemplate.Name).To(Equal("some-name"))
		Expect(runtimeConfigTemplate.RuntimeConfig).To(Equal("some-runtime-config"))
	})
})
