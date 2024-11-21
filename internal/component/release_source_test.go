package component_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("ReleaseSourceList", func() {
	Describe("NewReleaseSourceRepo", func() {
		var kilnfile cargo.Kilnfile

		Context("happy path", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{
						{Type: "s3", Bucket: "compiled-releases", Region: "us-west-1", Publishable: true, PathTemplate: "template"},
						{Type: "s3", Bucket: "built-releases", Region: "us-west-1", Publishable: false, PathTemplate: "template"},
						{Type: "bosh.io", Publishable: false},
						{Type: "github", Org: "cloudfoundry", GithubToken: "banana"},
					},
				}
			})

			It("constructs all the release sources", func() {
				releaseSources := component.NewReleaseSourceRepo(kilnfile)
				Expect(len(releaseSources)).To(Equal(4)) // not using HaveLen because S3 struct is so huge
			})

			It("constructs the compiled release source properly", func() {
				releaseSources := component.NewReleaseSourceRepo(kilnfile)
				Expect(releaseSources[0]).To(BeAssignableToTypeOf(component.S3ReleaseSource{}))
			})

			It("sets the release source id to bucket id for s3", func() {
				releaseSources := component.NewReleaseSourceRepo(kilnfile)

				Expect(releaseSources[0].Configuration().ID).To(Equal(kilnfile.ReleaseSources[0].Bucket))
				Expect(releaseSources[1].Configuration().ID).To(Equal(kilnfile.ReleaseSources[1].Bucket))
			})

			It("constructs the built release source properly", func() {
				releaseSources := component.NewReleaseSourceRepo(kilnfile)

				Expect(releaseSources[1]).To(BeAssignableToTypeOf(component.S3ReleaseSource{}))
				Expect(releaseSources[1].Configuration().ID).To(Equal(kilnfile.ReleaseSources[1].Bucket))
			})

			It("constructs the github release source properly", func() {
				releaseSources := component.NewReleaseSourceRepo(kilnfile)

				Expect(releaseSources[3]).To(BeAssignableToTypeOf(&component.GithubReleaseSource{}))
				Expect(releaseSources[3].Configuration().ID).To(Equal(kilnfile.ReleaseSources[3].Org))
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
				releaseSources := component.NewReleaseSourceRepo(kilnfile)

				Expect(releaseSources).To(HaveLen(1))
				var boshIOReleaseSource *component.BOSHIOReleaseSource

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(boshIOReleaseSource))
				Expect(releaseSources[0].(*component.BOSHIOReleaseSource).Publishable()).To(BeTrue())
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
				releaseSources := component.NewReleaseSourceRepo(kilnfile)

				Expect(releaseSources).To(HaveLen(3))
				Expect(releaseSources[0].Configuration().ID).To(Equal("comp"))
				Expect(releaseSources[1].Configuration().ID).To(Equal("buil"))
				Expect(releaseSources[2].Configuration().ID).To(Equal("bosh"))
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
				var r any
				func() {
					defer func() {
						r = recover()
					}()
					component.NewReleaseSourceRepo(kilnfile)
				}()
				Expect(r).To(ContainSubstring("unique"))
				Expect(r).To(ContainSubstring(`"some-bucket"`))
			})
		})
	})

	Describe("MultiReleaseSource", func() {
		var (
			list     component.ReleaseSourceList
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			list = component.NewReleaseSourceRepo(kilnfile)
		})

		Context("when allow-only-publishable-releases is false", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{
						{
							Type: "s3", Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
							PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
						},
						{
							Type: "s3", Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
							PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}.tgz`,
						},
						{Type: "bosh.io"},
						{
							Type: "s3", Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
							PathTemplate: `{{.Name}}-{{.Version}}.tgz`,
						},
					},
				}
			})

			It("builds the correct release sources", func() {
				releaseSources := list.Filter(false)
				Expect(releaseSources).To(HaveLen(4))
				var (
					s3ReleaseSource     component.S3ReleaseSource
					boshIOReleaseSource *component.BOSHIOReleaseSource
				)

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[0].Configuration().ID).To(Equal(kilnfile.ReleaseSources[0].Bucket))

				Expect(releaseSources[1]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[1].Configuration().ID).To(Equal(kilnfile.ReleaseSources[1].Bucket))

				Expect(releaseSources[2]).To(BeAssignableToTypeOf(boshIOReleaseSource))
				Expect(releaseSources[2].Configuration().ID).To(Equal("bosh.io"))

				Expect(releaseSources[3]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[3].Configuration().ID).To(Equal(kilnfile.ReleaseSources[3].Bucket))
			})
		})

		Context("when allow-only-publishable-releases is true", func() {
			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{
						{
							Publishable: true, Type: "s3", Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
							PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
						},
						{
							Type: "s3", Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
							PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}.tgz`,
						},
						{Type: "bosh.io"},
						{
							Type: "s3", Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
							PathTemplate: `{{.Name}}-{{.Version}}.tgz`,
						},
					},
				}
			})

			It("builds the correct release sources", func() {
				releaseSources := list.Filter(true)
				Expect(releaseSources).To(HaveLen(1))
				var s3ReleaseSource component.S3ReleaseSource

				Expect(releaseSources[0]).To(BeAssignableToTypeOf(s3ReleaseSource))
				Expect(releaseSources[0].Configuration().ID).To(Equal(kilnfile.ReleaseSources[0].Bucket))
				Expect(releaseSources[0].Configuration().Publishable).To(BeTrue())
			})
		})
	})

	Describe("FindReleaseUploader", func() {
		var (
			repo     component.ReleaseSourceList
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			repo = component.NewReleaseSourceRepo(kilnfile)
		})

		BeforeEach(func() {
			kilnfile = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{
						Type: "s3", Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
						PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
					},
					{
						Type: "s3", Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}.tgz`,
					},
					{Type: "bosh.io"},
					{
						Type: "s3", Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						PathTemplate: `{{.Name}}-{{.Version}}.tgz`,
					},
				},
			}
		})

		Context("when the named source exists and accepts uploads", func() {
			It("returns a valid release uploader", func() {
				uploader, err := repo.FindReleaseUploader("bucket-2")
				Expect(err).NotTo(HaveOccurred())

				var s3ReleaseSource component.S3ReleaseSource
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
			list     component.ReleaseSourceList
			kilnfile cargo.Kilnfile
		)

		JustBeforeEach(func() {
			list = component.NewReleaseSourceRepo(kilnfile)
		})

		BeforeEach(func() {
			kilnfile = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{
						Type: "s3", Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
						PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
					},
					{
						Type: "s3", Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}.tgz`,
					},
					{Type: "bosh.io"},
					{
						Type: "s3", Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
						PathTemplate: `{{.Name}}-{{.Version}}.tgz`,
					},
				},
			}
		})

		Context("when the named source exists and implements RemotePath", func() {
			It("returns a valid release uploader", func() {
				uploader, err := list.FindRemotePather("bucket-2")
				Expect(err).NotTo(HaveOccurred())

				var s3ReleaseSource component.S3ReleaseSource
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
				_, err := list.FindRemotePather("bosh.io")
				Expect(err).To(MatchError(ContainSubstring("no path-generating release sources were found")))
			})
		})

		Context("when the named source doesn't implement RemotePath", func() {
			It("errors with a list of valid sources", func() {
				_, err := list.FindRemotePather("bosh.io")
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})

		Context("when the named source doesn't exist", func() {
			It("errors with a list of valid sources", func() {
				_, err := list.FindRemotePather("bucket-42")
				Expect(err).To(MatchError(ContainSubstring("could not find a valid matching release source")))
				Expect(err).To(MatchError(ContainSubstring("bucket-1")))
				Expect(err).To(MatchError(ContainSubstring("bucket-2")))
				Expect(err).To(MatchError(ContainSubstring("bucket-3")))
			})
		})
	})
})
