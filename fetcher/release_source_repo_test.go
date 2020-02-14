package fetcher

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("ReleaseSourceRepo", func() {
	var logger *log.Logger

	BeforeEach(func() {
		logger = log.New(GinkgoWriter, "", log.LstdFlags)
	})

	Describe("NewReleaseSourceRepo", func() {
		var kilnfile cargo.Kilnfile

		Context("happy path", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{
						{Type: "s3", Bucket: "compiled-releases", Region: "us-west-1", Publishable: true},
						{Type: "s3", Bucket: "built-releases", Region: "us-west-1", Publishable: false},
						{Type: "bosh.io", Publishable: false},
					},
				}
			})

			It("constructs the ReleaseSources properly", func() {
				repo := NewReleaseSourceRepo(kilnfile, logger)
				releaseSources := repo.ReleaseSources

				Expect(releaseSources).To(HaveLen(3))
				var (
					s3ReleaseSource     S3ReleaseSource
					boshIOReleaseSource *BOSHIOReleaseSource
				)

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[0]).To(MatchFields(IgnoreExtras, Fields{
					"Bucket":       Equal(kilnfile.ReleaseSources[0].Bucket),
					"PathTemplate": Equal(kilnfile.ReleaseSources[0].PathTemplate),
				}))
				Expect(releaseSources[0].(S3ReleaseSource).Publishable()).To(BeTrue())

				Expect(releaseSources[1]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[1]).To(MatchFields(IgnoreExtras, Fields{
					"Bucket":       Equal(kilnfile.ReleaseSources[1].Bucket),
					"PathTemplate": Equal(kilnfile.ReleaseSources[1].PathTemplate),
				}))
				Expect(releaseSources[1].(S3ReleaseSource).Publishable()).To(BeFalse())

				Expect(releaseSources[2]).To(BeAssignableToTypeOf(boshIOReleaseSource))
				Expect(releaseSources[2].(*BOSHIOReleaseSource).Publishable()).To(BeFalse())
			})

		})

		Context("when bosh.io is publishable", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{
						{Type: "bosh.io", Publishable: true},
					},
				}
			})

			It("marks it correctly", func() {
				repo := NewReleaseSourceRepo(kilnfile, logger)
				releaseSources := repo.ReleaseSources

				Expect(releaseSources).To(HaveLen(1))
				var (
					boshIOReleaseSource *BOSHIOReleaseSource
				)

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(boshIOReleaseSource))
				Expect(releaseSources[0].(*BOSHIOReleaseSource).Publishable()).To(BeTrue())
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

			It("panics with a helpful message", func() {
				var r interface{}
				func() {
					defer func() {
						r = recover()
					}()
					NewReleaseSourceRepo(kilnfile, logger)
				}()
				Expect(r).To(ContainSubstring("unique"))
				Expect(r).To(ContainSubstring(`"some-bucket"`))
			})
		})
	})

	Describe("MultiReleaseSource", func() {
		var (
			repo     ReleaseSourceRepo
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			repo = NewReleaseSourceRepo(kilnfile, logger)
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
				releaseSources := repo.MultiReleaseSource(false).(multiReleaseSource)
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
						{Type: "s3", Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
							PathTemplate: `{{.Name}}-{{.Version}}.tgz`},
					},
				}
			})

			It("builds the correct release sources", func() {
				releaseSources := repo.MultiReleaseSource(true).(multiReleaseSource)
				Expect(releaseSources).To(HaveLen(1))
				var s3ReleaseSource S3ReleaseSource

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[0]).To(MatchFields(IgnoreExtras, Fields{
					"Bucket":       Equal(kilnfile.ReleaseSources[0].Bucket),
					"PathTemplate": Equal(kilnfile.ReleaseSources[0].PathTemplate),
				}))

				releaseSource, ok := releaseSources[0].(ReleaseSource)
				Expect(ok).To(BeTrue(), "Couldn't convert releaseSources[0] to type ReleaseSource")
				Expect(releaseSource.Publishable()).To(BeTrue())
			})
		})
	})

	Describe("FindReleaseUploader", func() {
		var (
			repo     ReleaseSourceRepo
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			repo = NewReleaseSourceRepo(kilnfile, logger)
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
				uploader, err := repo.FindReleaseUploader("bucket-2")
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
				_, err := repo.FindReleaseUploader("bosh.io")
				Expect(err).To(MatchError(ContainSubstring("no upload-capable release sources were found")))
			})
		})

		Context("when the named source doesn't accept uploads", func() {
			It("errors with a list of valid sources", func() {
				_, err := repo.FindReleaseUploader("bosh.io")
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})

		Context("when the named source doesn't exist", func() {
			It("errors with a list of valid sources", func() {
				_, err := repo.FindReleaseUploader("bucket-42")
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})
	})

	Describe("RemotePather", func() {
		var (
			repo     ReleaseSourceRepo
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			repo = NewReleaseSourceRepo(kilnfile, logger)
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
				uploader, err := repo.FindRemotePather("bucket-2")
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
				_, err := repo.FindRemotePather("bosh.io")
				Expect(err).To(MatchError(ContainSubstring("no path-generating release sources were found")))
			})
		})

		Context("when the named source doesn't implement RemotePath", func() {
			It("errors with a list of valid sources", func() {
				_, err := repo.FindRemotePather("bosh.io")
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})

		Context("when the named source doesn't exist", func() {
			It("errors with a list of valid sources", func() {
				_, err := repo.FindRemotePather("bucket-42")
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})
	})
})
