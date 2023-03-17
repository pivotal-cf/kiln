package cargo_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestInterpolateAndParseKilnfile(t *testing.T) {
	please := NewWithT(t)

	variables := map[string]interface{}{
		"bucket":        "my-bucket",
		"region":        "middle-earth",
		"path_template": "not-used",

		"access_key": "id",
		"secret_key": "key",
	}

	kilnfile, err := cargo.InterpolateAndParseKilnfile(
		strings.NewReader(validKilnfile), variables,
	)

	please.Expect(err).NotTo(HaveOccurred())

	please.Expect(kilnfile).To(Equal(cargo.Kilnfile{
		ReleaseSources: []cargo.ReleaseSourceConfig{
			{
				Type:            "s3",
				Bucket:          "my-bucket",
				Region:          "middle-earth",
				AccessKeyID:     "id",
				SecretAccessKey: "key",
				PathTemplate:    "not-used",
			},
		},
	}))
}

func TestInterpolateAndParseKilnfile_input_is_not_valid_yaml(t *testing.T) {
	please := NewWithT(t)

	variables := map[string]interface{}{
		"bucket":        "my-bucket",
		"region":        "middle-earth",
		"path_template": "not-used",

		"access_key": "id",
		"secret_key": "key",
	}

	_, err := cargo.InterpolateAndParseKilnfile(
		strings.NewReader("invalid : bad : yaml"), variables,
	)

	please.Expect(err).To(HaveOccurred())
}

func TestInterpolateAndParseKilnfile_interpolation_variable_not_found(t *testing.T) {
	please := NewWithT(t)

	variables := map[string]interface{}{
		"bucket": "my-bucket",
		// "region": "middle-earth", // <- the missing variable
		"path_template": "not-used",

		"access_key": "id",
		"secret_key": "key",
	}

	_, err := cargo.InterpolateAndParseKilnfile(
		strings.NewReader(validKilnfile), variables,
	)

	please.Expect(err).To(HaveOccurred())
}

const validKilnfile = `---
release_sources:
  - type: s3
    compiled: true
    bucket: $( variable "bucket" )
    region: $( variable "region" )
    access_key_id: $( variable "access_key" )
    secret_access_key: $( variable "secret_key" )
    path_template: $( variable "path_template" )
`
