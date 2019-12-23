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
		kilnfile  cargo.Kilnfile
	)

	JustBeforeEach(func() {
		rsFactory = NewReleaseSourcesFactory(log.New(GinkgoWriter, "", log.LstdFlags))
	})

	Context("when allow-only-publishable-releases is false", func() {
		BeforeEach(func() {
			kilnfile = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{Type: "s3", Compiled: true, Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
						Regex: `^2.8/.+/(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>\d+\.\d+)(\.0)?\.tgz$`},
					{Type: "s3", Compiled: false, Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						Regex: `^2.8/.+/(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\.]+)\.tgz$`},
					{Type: "bosh.io"},
					{Type: "s3", Compiled: false, Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						Regex: `^(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\.]+-?[a-zA-Z0-9]\.?[0-9]*)\.tgz$`},
				},
			}
		})

		It("builds the correct release sources", func() {
			releaseSources := rsFactory.ReleaseSources(kilnfile, false)
			Expect(releaseSources).To(HaveLen(4))
			var (
				s3CompiledReleaseSource S3CompiledReleaseSource
				s3BuiltReleaseSource    S3BuiltReleaseSource
				boshIOReleaseSource     *BOSHIOReleaseSource
			)

			Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3CompiledReleaseSource))
			Expect(releaseSources[0]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket": Equal(kilnfile.ReleaseSources[0].Bucket),
				"Regex":  Equal(kilnfile.ReleaseSources[0].Regex),
			}))

			Expect(releaseSources[1]).To(BeAssignableToTypeOf(s3BuiltReleaseSource))
			Expect(releaseSources[1]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket": Equal(kilnfile.ReleaseSources[1].Bucket),
				"Regex":  Equal(kilnfile.ReleaseSources[1].Regex),
			}))

			Expect(releaseSources[2]).To(BeAssignableToTypeOf(boshIOReleaseSource))

			Expect(releaseSources[3]).To(BeAssignableToTypeOf(s3BuiltReleaseSource))
			Expect(releaseSources[3]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket": Equal(kilnfile.ReleaseSources[3].Bucket),
				"Regex":  Equal(kilnfile.ReleaseSources[3].Regex),
			}))
		})
	})

	Context("when allow-only-publishable-releases is true", func() {
		BeforeEach(func() {
			kilnfile = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{Publishable: true, Type: "s3", Compiled: true, Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
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
			releaseSources := rsFactory.ReleaseSources(kilnfile, true)
			Expect(releaseSources).To(HaveLen(1))
			var s3CompiledReleaseSource S3CompiledReleaseSource

			Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3CompiledReleaseSource))
			Expect(releaseSources[0]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket": Equal(kilnfile.ReleaseSources[0].Bucket),
				"Regex":  Equal(kilnfile.ReleaseSources[0].Regex),
			}))
		})
	})

	Context("when there are duplicate release source identifiers", func() {
		BeforeEach(func() {
			kilnfile = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{Type: "s3", Compiled: true, Bucket: "some-bucket", Region: "us-west-1"},
					{Type: "s3", Compiled: false, Bucket: "some-bucket", Region: "us-west-1"},
				},
			}
		})

		It("builds the correct release sources", func() {
			var r interface{}
			func() {
				defer func() {
					r = recover()
				}()
				rsFactory.ReleaseSources(kilnfile, false)
			}()
			Expect(r).To(ContainSubstring("unique"))
			Expect(r).To(ContainSubstring(`"some-bucket"`))
		})
	})
})
