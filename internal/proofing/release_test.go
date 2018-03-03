package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Release", func() {
	var release proofing.Release

	BeforeEach(func() {
		metadata, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		release = metadata.Releases[0]
	})

	It("parses their structure", func() {
		Expect(release.File).To(Equal("some-file"))
		Expect(release.Name).To(Equal("some-name"))
		Expect(release.SHA1).To(Equal("some-sha1"))
		Expect(release.Version).To(Equal("some-version"))
	})
})
