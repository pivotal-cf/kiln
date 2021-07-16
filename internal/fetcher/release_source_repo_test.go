package fetcher_test

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/fetcher"
	"github.com/pivotal-cf/kiln/pkg/cargo"
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
						{Type: "s3", Bucket: "compiled-releases", Region: "us-west-1", Publishable: true, PathTemplate: "template"},
						{Type: "s3", Bucket: "built-releases", Region: "us-west-1", Publishable: false, PathTemplate: "template"},
						{Type: "bosh.io", Publishable: false},
					},
				}
			})

			It("constructs the ReleaseSources properly", func() {
				repo := fetcher.NewReleaseSourceRepo(kilnfile, logger)
				releaseSources := repo.ReleaseSources

				Expect(releaseSources).To(HaveLen(3))
				var (
					s3ReleaseSource     fetcher.S3ReleaseSource
					boshIOReleaseSource *fetcher.BOSHIOReleaseSource
				)

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[0].ID()).To(Equal(kilnfile.ReleaseSources[0].Bucket))
				Expect(releaseSources[0].Publishable()).To(BeTrue())

				Expect(releaseSources[1]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[1].ID()).To(Equal(kilnfile.ReleaseSources[1].Bucket))
				Expect(releaseSources[1].Publishable()).To(BeFalse())

				Expect(releaseSources[2]).To(BeAssignableToTypeOf(boshIOReleaseSource))
				Expect(releaseSources[2].ID()).To(Equal("bosh.io"))
				Expect(releaseSources[2].Publishable()).To(BeFalse())
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
				repo := fetcher.NewReleaseSourceRepo(kilnfile, logger)
				releaseSources := repo.ReleaseSources

				Expect(releaseSources).To(HaveLen(1))
				var (
					boshIOReleaseSource *fetcher.BOSHIOReleaseSource
				)

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(boshIOReleaseSource))
				Expect(releaseSources[0].(*fetcher.BOSHIOReleaseSource).Publishable()).To(BeTrue())
			})
		})

		Context("when the Kilnfile gives explicit IDs", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{
						{ID: "comp", Type: "s3", Bucket: "compiled-releases", Region: "us-west-1", Publishable: true, PathTemplate: "template"},
						{ID: "buil", Type: "s3", Bucket: "built-releases", Region: "us-west-1", Publishable: false, PathTemplate: "template"},
						{ID: "bosh", Type: "bosh.io", Publishable: false},
					},
				}
			})

			It("gives the correct IDs to the release sources", func() {
				repo := fetcher.NewReleaseSourceRepo(kilnfile, logger)
				releaseSources := repo.ReleaseSources

				Expect(releaseSources).To(HaveLen(3))
				Expect(releaseSources[0].ID()).To(Equal("comp"))
				Expect(releaseSources[1].ID()).To(Equal("buil"))
				Expect(releaseSources[2].ID()).To(Equal("bosh"))
			})
		})

		Context("when there are duplicate release source identifiers", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{
						{Type: "s3", Bucket: "some-bucket", Region: "us-west-1", PathTemplate: "template"},
						{Type: "s3", Bucket: "some-bucket", Region: "us-west-1", PathTemplate: "template"},
					},
				}
			})

			It("panics with a helpful message", func() {
				var r interface{}
				func() {
					defer func() {
						r = recover()
					}()
					fetcher.NewReleaseSourceRepo(kilnfile, logger)
				}()
				Expect(r).To(ContainSubstring("unique"))
				Expect(r).To(ContainSubstring(`"some-bucket"`))
			})
		})
	})

	Describe("MultiReleaseSource", func() {
		var (
			repo     fetcher.ReleaseSourceRepo
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			repo = fetcher.NewReleaseSourceRepo(kilnfile, logger)
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
				releaseSources := repo.MultiReleaseSource(false)
				Expect(releaseSources).To(HaveLen(4))
				var (
					s3ReleaseSource     fetcher.S3ReleaseSource
					boshIOReleaseSource *fetcher.BOSHIOReleaseSource
				)

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[0].ID()).To(Equal(kilnfile.ReleaseSources[0].Bucket))

				Expect(releaseSources[1]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[1].ID()).To(Equal(kilnfile.ReleaseSources[1].Bucket))

				Expect(releaseSources[2]).To(BeAssignableToTypeOf(boshIOReleaseSource))
				Expect(releaseSources[2].ID()).To(Equal("bosh.io"))

				Expect(releaseSources[3]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[3].ID()).To(Equal(kilnfile.ReleaseSources[3].Bucket))
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
				releaseSources := repo.MultiReleaseSource(true)
				Expect(releaseSources).To(HaveLen(1))
				var s3ReleaseSource fetcher.S3ReleaseSource

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[0].ID()).To(Equal(kilnfile.ReleaseSources[0].Bucket))
				Expect(releaseSources[0].Publishable()).To(BeTrue())
			})
		})
	})

	Describe("FindReleaseUploader", func() {
		var (
			repo     fetcher.ReleaseSourceRepo
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			repo = fetcher.NewReleaseSourceRepo(kilnfile, logger)
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

				var s3ReleaseSource fetcher.S3ReleaseSource
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
			repo     fetcher.ReleaseSourceRepo
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			repo = fetcher.NewReleaseSourceRepo(kilnfile, logger)
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

				var s3ReleaseSource fetcher.S3ReleaseSource
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
