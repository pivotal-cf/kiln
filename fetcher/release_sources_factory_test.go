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

var _ = Describe("ReleaseSourceFactory", func() {
	Describe("ReleaseSource", func() {
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
				releaseSources := rsFactory.ReleaseSource(kilnfile, false).(MultiReleaseSource)
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
				releaseSources := rsFactory.ReleaseSource(kilnfile, true).(MultiReleaseSource)
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

	Describe("ReleaseUploader", func() {
		var (
			ruFactory commands.ReleaseUploaderFactory
			kilnfile  cargo.Kilnfile
		)

		JustBeforeEach(func() {
			ruFactory = NewReleaseSourceFactory(log.New(GinkgoWriter, "", log.LstdFlags))
		})

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

		Context("when the named source exists and accepts uploads", func() {
			It("returns a valid release uploader", func() {
				uploader, err := ruFactory.ReleaseUploader("bucket-2", kilnfile)
				Expect(err).NotTo(HaveOccurred())

				var s3ReleaseSource S3ReleaseSource
				Expect(uploader).To(BeAssignableToTypeOf(s3ReleaseSource))
			})
		})

		Context("when no sources accept uploads", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{{Type: "bosh.io"}},
				}
			})

			It("errors", func() {
				_, err := ruFactory.ReleaseUploader("bosh.io", kilnfile)
				Expect(err).To(MatchError(ContainSubstring("no upload-capable release sources were found")))
			})
		})

		Context("when the named source doesn't accept uploads", func() {
			It("errors with a list of valid sources", func() {
				_, err := ruFactory.ReleaseUploader("bosh.io", kilnfile)
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})

		Context("when the named source doesn't exist", func() {
			It("errors with a list of valid sources", func() {
				_, err := ruFactory.ReleaseUploader("bucket-42", kilnfile)
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})
	})

	Describe("RemotePather", func() {
		var (
			rpFactory commands.RemotePatherFactory
			kilnfile  cargo.Kilnfile
		)

		JustBeforeEach(func() {
			rpFactory = NewReleaseSourceFactory(log.New(GinkgoWriter, "", log.LstdFlags))
		})

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

		Context("when the named source exists and implements RemotePath", func() {
			It("returns a valid release uploader", func() {
				uploader, err := rpFactory.RemotePather("bucket-2", kilnfile)
				Expect(err).NotTo(HaveOccurred())

				var s3ReleaseSource S3ReleaseSource
				Expect(uploader).To(BeAssignableToTypeOf(s3ReleaseSource))
			})
		})

		Context("when no sources implement RemotePath", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{{Type: "bosh.io"}},
				}
			})

			It("errors", func() {
				_, err := rpFactory.RemotePather("bosh.io", kilnfile)
				Expect(err).To(MatchError(ContainSubstring("no path-generating release sources were found")))
			})
		})

		Context("when the named source doesn't implement RemotePath", func() {
			It("errors with a list of valid sources", func() {
				_, err := rpFactory.RemotePather("bosh.io", kilnfile)
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})

		Context("when the named source doesn't exist", func() {
			It("errors with a list of valid sources", func() {
				_, err := rpFactory.RemotePather("bucket-42", kilnfile)
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})
	})
})
