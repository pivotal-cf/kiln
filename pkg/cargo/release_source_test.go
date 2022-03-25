package cargo_test

import (
	"testing"

	Ω "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestReleaseSourceList_FilterOnAllowPublish(t *testing.T) {
	t.Run("when allow-only-publishable-releases is false", func(t *testing.T) {
		please := Ω.NewWithT(t)

		kilnfile := cargo.Kilnfile{
			ReleaseSources: cargo.ReleaseSourceList{
				cargo.S3ReleaseSource{Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
					PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`},
				cargo.S3ReleaseSource{Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
					PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}.tgz`},
				cargo.BOSHIOReleaseSource{},
				cargo.S3ReleaseSource{Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
					PathTemplate: `{{.Name}}-{{.Version}}.tgz`},
			},
		}

		releaseSources := kilnfile.ReleaseSources.Filter(false)

		please.Expect(releaseSources).To(Ω.HaveLen(4))
		var (
			s3ReleaseSource     cargo.S3ReleaseSource
			boshIOReleaseSource cargo.BOSHIOReleaseSource
		)

		please.Expect(releaseSources[0]).To(Ω.BeAssignableToTypeOf(s3ReleaseSource))
		please.Expect(releaseSources[0].ID()).To(Ω.Equal(kilnfile.ReleaseSources[0].(cargo.S3ReleaseSource).Bucket))

		please.Expect(releaseSources[1]).To(Ω.BeAssignableToTypeOf(s3ReleaseSource))
		please.Expect(releaseSources[1].ID()).To(Ω.Equal(kilnfile.ReleaseSources[1].(cargo.S3ReleaseSource).Bucket))

		please.Expect(releaseSources[2]).To(Ω.BeAssignableToTypeOf(boshIOReleaseSource))
		please.Expect(releaseSources[2].ID()).To(Ω.Equal("bosh.io"))

		please.Expect(releaseSources[3]).To(Ω.BeAssignableToTypeOf(s3ReleaseSource))
		please.Expect(releaseSources[3].ID()).To(Ω.Equal(kilnfile.ReleaseSources[3].(cargo.S3ReleaseSource).Bucket))
	})

	t.Run("when allow-only-publishable-releases is true", func(t *testing.T) {
		please := Ω.NewWithT(t)

		kilnfile := cargo.Kilnfile{
			ReleaseSources: cargo.ReleaseSourceList{
				cargo.S3ReleaseSource{Publishable: true, Bucket: "bucket-1", Region: "us-west-1", AccessKeyId: "ak1", SecretAccessKey: "shhhh!",
					PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
				},
				cargo.S3ReleaseSource{Bucket: "bucket-2", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
					PathTemplate: `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}.tgz`,
				},
				cargo.BOSHIOReleaseSource{},
				cargo.S3ReleaseSource{Bucket: "bucket-3", Region: "us-west-2", AccessKeyId: "aki", SecretAccessKey: "shhhh!",
					PathTemplate: `{{.Name}}-{{.Version}}.tgz`},
			},
		}

		releaseSources := kilnfile.ReleaseSources.Filter(true)

		please.Expect(releaseSources).To(Ω.HaveLen(1))
		var s3ReleaseSource cargo.S3ReleaseSource

		please.Expect(releaseSources[0]).To(Ω.BeAssignableToTypeOf(s3ReleaseSource))
		please.Expect(releaseSources[0].ID()).To(Ω.Equal(kilnfile.ReleaseSources[0].(cargo.S3ReleaseSource).Bucket))
		please.Expect(releaseSources[0].IsPublishable()).To(Ω.BeTrue())
	})
}

func TestReleaseSourceList_Validate(t *testing.T) {
	please := Ω.NewWithT(t)

	kilnfile := cargo.Kilnfile{
		ReleaseSources: cargo.ReleaseSourceList{
			cargo.S3ReleaseSource{Bucket: "some-bucket", Region: "us-west-1", PathTemplate: "template"},
			cargo.S3ReleaseSource{Bucket: "some-bucket", Region: "us-west-1", PathTemplate: "template"},
		},
	}

	err := kilnfile.ReleaseSources.Validate()
	please.Expect(err).To(Ω.HaveOccurred())
	please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("unique")))
	please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring(`"some-bucket"`)))
}
