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

func TestReleaseSourceList_ConfigureSecrets(t *testing.T) {
	please := Ω.NewWithT(t)

	t.Setenv("GITHUB_TOKEN", "env-gh-tok")
	t.Setenv("AWS_ACCESS_KEY_ID", "env-aws-id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "env-aws-key")

	kilnfile := cargo.Kilnfile{
		ReleaseSources: cargo.ReleaseSourceList{
			cargo.BOSHIOReleaseSource{},
			cargo.S3ReleaseSource{
				// already set
				SecretAccessKey: "shhhh!",
				AccessKeyId:     "hello",
			},
			cargo.S3ReleaseSource{
				// load from env
				SecretAccessKey: "",
				AccessKeyId:     "",
			},
			cargo.S3ReleaseSource{
				// interpolate template
				SecretAccessKey: ` $( variable "aws_sak" )`,
				AccessKeyId:     `$( variable  "aws_aki" )`,
			},
			cargo.S3ReleaseSource{
				// interpolate template
				SecretAccessKey: `$( variable "not_set" )`,
				AccessKeyId:     `$( variable "not_set" )`,
			},

			cargo.GitHubReleaseSource{},
			cargo.GitHubReleaseSource{
				// not set
				GithubToken: `           $(variable "gh_tok")`,
			},
			cargo.GitHubReleaseSource{
				// not set
				GithubToken: `$( variable "not_set" )`,
			},
			cargo.GitHubReleaseSource{
				// not set
				GithubToken: `$( `,
			},
		},
	}

	tv := map[string]interface{}{
		"gh_tok":  "tem-gh-token",
		"aws_aki": "tem-aws-id",
		"aws_sak": "tem-aws-key",
	}

	succeeded, failed, errList := kilnfile.ReleaseSources.ConfigureSecrets(tv)

	please.Expect(succeeded).To(Ω.Equal(cargo.ReleaseSourceList{
		cargo.BOSHIOReleaseSource{},
		cargo.S3ReleaseSource{
			// already set
			SecretAccessKey: "shhhh!",
			AccessKeyId:     "hello",
		},
		cargo.S3ReleaseSource{
			// load from env
			AccessKeyId:     "env-aws-id",
			SecretAccessKey: "env-aws-key",
		},
		cargo.S3ReleaseSource{
			// interpolate template
			AccessKeyId:     `tem-aws-id`,
			SecretAccessKey: `tem-aws-key`,
		},
		cargo.GitHubReleaseSource{
			// not set
			GithubToken: `env-gh-tok`,
		},
		cargo.GitHubReleaseSource{
			// not set
			GithubToken: `tem-gh-token`,
		},
	}))

	please.Expect(failed).To(Ω.HaveLen(3))
	please.Expect(errList).To(Ω.HaveLen(len(failed)))
}
