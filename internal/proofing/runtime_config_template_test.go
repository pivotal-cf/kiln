package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuntimeConfigTemplate", func() {
	var runtimeConfigTemplate proofing.RuntimeConfigTemplate

	BeforeEach(func() {
		metadata, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		runtimeConfigTemplate = metadata.RuntimeConfigs[0]
	})

	It("parses their structure", func() {
		Expect(runtimeConfigTemplate.Name).To(Equal("some-name"))
		Expect(runtimeConfigTemplate.RuntimeConfig).To(Equal("some-runtime-config"))
	})
})
