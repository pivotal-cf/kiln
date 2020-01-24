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

var _ = Describe("NewReleaseSourceFactory()", func() {
	var (
		rsFactory commands.ReleaseSourceFactory
		kilnfile  cargo.Kilnfile
	)

	JustBeforeEach(func() {
		rsFactory = NewReleaseSourceFactory(log.New(GinkgoWriter, "", log.LstdFlags))
	})

	Context("when allow-only-publishable-releases is false", func() {
		BeforeEach(func() {
			kilnfile = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{Type: "s3", Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
						PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`},
					{Type: "s3", Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}.tgz`},
					{Type: "bosh.io"},
					{Type: "s3", Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						PathTemplate: `{{.Name}}-{{.Version}}.tgz`},
				},
			}
		})

		It("builds the correct release sources", func() {
			releaseSources := rsFactory.ReleaseSource(kilnfile, false)
			Expect(releaseSources).To(HaveLen(4))
			var (
				s3ReleaseSource     S3ReleaseSource
				boshIOReleaseSource *BOSHIOReleaseSource
			)

			Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
			Expect(releaseSources[0]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket":       Equal(kilnfile.ReleaseSources[0].Bucket),
				"PathTemplate": Equal(kilnfile.ReleaseSources[0].PathTemplate),
			}))

			Expect(releaseSources[1]).To(BeAssignableToTypeOf(s3ReleaseSource))
			Expect(releaseSources[1]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket":       Equal(kilnfile.ReleaseSources[1].Bucket),
				"PathTemplate": Equal(kilnfile.ReleaseSources[1].PathTemplate),
			}))

			Expect(releaseSources[2]).To(BeAssignableToTypeOf(boshIOReleaseSource))

			Expect(releaseSources[3]).To(BeAssignableToTypeOf(s3ReleaseSource))
			Expect(releaseSources[3]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket":       Equal(kilnfile.ReleaseSources[3].Bucket),
				"PathTemplate": Equal(kilnfile.ReleaseSources[3].PathTemplate),
			}))
		})
	})

	Context("when allow-only-publishable-releases is true", func() {
		BeforeEach(func() {
			kilnfile = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{Publishable: true, Type: "s3", Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
						PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`},
					{Type: "s3", Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}.tgz`},
					{Type: "bosh.io"},
					{Type: "s3", Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						PathTemplate: `{{.Name}}-{{.Version}}.tgz`},
				},
			}
		})

		It("builds the correct release sources", func() {
			releaseSources := rsFactory.ReleaseSource(kilnfile, true)
			Expect(releaseSources).To(HaveLen(1))
			var s3ReleaseSource S3ReleaseSource

			Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
			Expect(releaseSources[0]).To(MatchFields(IgnoreExtras, Fields{
				"Bucket":       Equal(kilnfile.ReleaseSources[0].Bucket),
				"PathTemplate": Equal(kilnfile.ReleaseSources[0].PathTemplate),
			}))
		})
	})

	Context("when there are duplicate release source identifiers", func() {
		BeforeEach(func() {
			kilnfile = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{Type: "s3", Bucket: "some-bucket", Region: "us-west-1"},
					{Type: "s3", Bucket: "some-bucket", Region: "us-west-1"},
				},
			}
		})

		It("builds the correct release sources", func() {
			var r interface{}
			func() {
				defer func() {
					r = recover()
				}()
				rsFactory.ReleaseSource(kilnfile, false)
			}()
			Expect(r).To(ContainSubstring("unique"))
			Expect(r).To(ContainSubstring(`"some-bucket"`))
		})
	})
})
