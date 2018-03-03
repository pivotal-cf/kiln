package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProductTemplate", func() {
	var metadata proofing.ProductTemplate

	BeforeEach(func() {
		var err error
		metadata, err = proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())
	})

	It("parses a metadata file", func() {
		Expect(metadata.Description).To(Equal("some-description"))
		Expect(metadata.IconImage).To(Equal("some-icon-image"))
		Expect(metadata.Label).To(Equal("some-label"))
		Expect(metadata.MetadataVersion).To(Equal("some-metadata-version"))
		Expect(metadata.MinimumVersionForUpgrade).To(Equal("some-minimum-version-for-upgrade"))
		Expect(metadata.Name).To(Equal("some-name"))
		Expect(metadata.ProductVersion).To(Equal("some-product-version"))
		Expect(metadata.Rank).To(Equal(1))
		Expect(metadata.Serial).To(BeTrue())
		Expect(metadata.OriginalMetadataVersion).To(Equal("some-original-metadata-version"))
		Expect(metadata.ServiceBroker).To(BeTrue())
		Expect(metadata.DeprecatedTileImage).To(Equal("some-deprecated-tile-image"))

		Expect(metadata.FormTypes).To(HaveLen(1))
		Expect(metadata.InstallTimeVerifiers).To(HaveLen(1))
		Expect(metadata.JobTypes).To(HaveLen(1))
		Expect(metadata.PostDeployErrands).To(HaveLen(1))
		Expect(metadata.PropertyBlueprints).To(HaveLen(1))
		Expect(metadata.ProvidesProductVersions).To(HaveLen(1))
		Expect(metadata.Releases).To(HaveLen(1))
		Expect(metadata.RuntimeConfigs).To(HaveLen(1))
		Expect(metadata.StemcellCriteria).To(BeAssignableToTypeOf(proofing.StemcellCriteria{}))
		Expect(metadata.Variables).To(HaveLen(1))
	})
})
