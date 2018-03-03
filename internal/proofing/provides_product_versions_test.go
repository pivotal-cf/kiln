package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProvidesProductVersions", func() {
	var providesProductVersion proofing.ProvidesProductVersion

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		providesProductVersion = productTemplate.ProvidesProductVersions[0]
	})

	It("parses their structure", func() {
		Expect(providesProductVersion.Name).To(Equal("some-name"))
		Expect(providesProductVersion.Version).To(Equal("some-version"))
	})
})
