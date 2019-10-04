package fetcher_test

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/pivotal-cf/kiln/commands"
	. "github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("NewReleaseSourcesFactory()", func() {
	var (
		rsFactory commands.ReleaseSourcesFactory
		assets    cargo.Assets
	)

	JustBeforeEach(func() {
		rsFactory = NewReleaseSourcesFactory(log.New(GinkgoWriter, "", log.LstdFlags))
	})

	Context("on the happy path", func() {
		BeforeEach(func() {
			assets = cargo.Assets{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{Type: "s3", Compiled: true, Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
						Regex: `^2.8/.+/(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>\d+\.\d+)(\.0)?\.tgz$`},
					{Type: "s3", Compiled: false, Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						Regex: `^2.8/.+/(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\.]+)\.tgz$`},
					{Type: "bosh.io"},
					{Type: "s3", Compiled: false, Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						Regex: `^(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\.]+-?[a-zA-Z0-9]\.?[0-9]*)\.tgz$`},
				},
			}
		})

		It("builds the correct release sources", func() {
			releaseSources := rsFactory.ReleaseSources(assets)
			Expect(releaseSources).To(HaveLen(4))
			var (
				s3CompiledReleaseSource S3CompiledReleaseSource
				s3BuiltReleaseSource    S3BuiltReleaseSource
				boshIOReleaseSource     *BOSHIOReleaseSource
			)

			Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3CompiledReleaseSource))
			Expect(releaseSources[0]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket": Equal(assets.ReleaseSources[0].Bucket),
				"Regex":  Equal(assets.ReleaseSources[0].Regex),
			}))

			Expect(releaseSources[1]).To(BeAssignableToTypeOf(s3BuiltReleaseSource))
			Expect(releaseSources[1]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket": Equal(assets.ReleaseSources[1].Bucket),
				"Regex":  Equal(assets.ReleaseSources[1].Regex),
			}))

			Expect(releaseSources[2]).To(BeAssignableToTypeOf(boshIOReleaseSource))

			Expect(releaseSources[3]).To(BeAssignableToTypeOf(s3BuiltReleaseSource))
			Expect(releaseSources[3]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket": Equal(assets.ReleaseSources[3].Bucket),
				"Regex":  Equal(assets.ReleaseSources[3].Regex),
			}))
		})
	})
})
