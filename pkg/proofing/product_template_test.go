package proofing_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("ProductTemplate", func() {
	var productTemplate proofing.ProductTemplate

	BeforeEach(func() {
		f, err := os.Open("testdata/metadata.yml")
		defer closeAndIgnoreError(f)
		Expect(err).NotTo(HaveOccurred())

		productTemplate, err = proofing.Parse(f)
		Expect(err).NotTo(HaveOccurred())
	})

	It("parses a metadata file", func() {
		Expect(productTemplate.IconImage).To(Equal("some-icon-image"))
		Expect(productTemplate.Label).To(Equal("some-label"))
		Expect(productTemplate.MetadataVersion).To(Equal("some-metadata-version"))
		Expect(productTemplate.MinimumVersionForUpgrade).To(Equal("some-minimum-version-for-upgrade"))
		Expect(productTemplate.Name).To(Equal("some-name"))
		Expect(productTemplate.ProductVersion).To(Equal("some-product-version"))
		Expect(productTemplate.Rank).To(Equal(1))
		Expect(productTemplate.Serial).To(BeTrue())
		Expect(productTemplate.OriginalMetadataVersion).To(Equal("some-original-metadata-version"))
		Expect(productTemplate.ServiceBroker).To(BeTrue())
		Expect(productTemplate.DeprecatedTileImage).To(Equal("some-deprecated-tile-image"))
		Expect(productTemplate.BaseReleasesURL).To(Equal("some-base-releases-url"))
		Expect(productTemplate.Cloud).To(Equal("some-cloud"))
		Expect(productTemplate.Network).To(Equal("some-network"))

		Expect(productTemplate.FormTypes).To(HaveLen(1))
		Expect(productTemplate.InstallTimeVerifiers).To(HaveLen(1))
		Expect(productTemplate.JobTypes).To(HaveLen(1))
		Expect(productTemplate.PostDeployErrands).To(HaveLen(1))
		Expect(productTemplate.PreDeleteErrands).To(HaveLen(1))
		Expect(productTemplate.PropertyBlueprints).To(HaveLen(1))
		Expect(productTemplate.RequiresProductVersions).To(HaveLen(1))
		Expect(productTemplate.Releases).To(HaveLen(1))
		Expect(productTemplate.RuntimeConfigs).To(HaveLen(1))
		Expect(productTemplate.StemcellCriteria).To(BeAssignableToTypeOf(proofing.StemcellCriteria{}))
		Expect(productTemplate.Variables).To(HaveLen(1))
	})
})
