package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Template", func() {
	var template proofing.Template

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		template = productTemplate.JobTypes[0].Templates[0]
	})

	It("parses their structure", func() {
		Expect(template.Consumes).To(Equal("some-consumes"))
		Expect(template.Manifest).To(Equal("some-manifest"))
		Expect(template.Name).To(Equal("some-name"))
		Expect(template.Provides).To(Equal("some-provides"))
		Expect(template.Release).To(Equal("some-release"))
	})
})
