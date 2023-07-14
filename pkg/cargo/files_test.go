package cargo_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
				AccessKeyId:     "id",
				SecretAccessKey: "key",
				PathTemplate:    "not-used",
			},
		},
	}))

	t.Run("reading fails", func(t *testing.T) {
		r := iotest.ErrReader(errors.New("lemon"))
		_, err := cargo.InterpolateAndParseKilnfile(r, make(map[string]any))
		assert.Error(t, err)
	})
}

func TestInterpolateAndParseKilnfileWithRoleARN(t *testing.T) {
	please := NewWithT(t)

	const validKilnfileWithRoleARN = `---
release_sources:
  - type: s3
    compiled: true
    bucket: $( variable "bucket" )
    region: $( variable "region" )
    access_key_id: $( variable "access_key" )
    secret_access_key: $( variable "secret_key" )
    role_arn: $( variable "role_arn" )
    path_template: $( variable "path_template" )
`

	variables := map[string]interface{}{
		"bucket":        "my-bucket",
		"region":        "middle-earth",
		"path_template": "not-used",

		"access_key": "id",
		"secret_key": "key",
		"role_arn":   "role-arn",
	}

	kilnfile, err := cargo.InterpolateAndParseKilnfile(
		strings.NewReader(validKilnfileWithRoleARN), variables,
	)

	please.Expect(err).NotTo(HaveOccurred())

	please.Expect(kilnfile).To(Equal(cargo.Kilnfile{
		ReleaseSources: []cargo.ReleaseSourceConfig{
			{
				Type:            "s3",
				Bucket:          "my-bucket",
				Region:          "middle-earth",
				AccessKeyId:     "id",
				RoleARN:         "role-arn",
				SecretAccessKey: "key",
				PathTemplate:    "not-used",
			},
		},
	}))

	t.Run("reading fails", func(t *testing.T) {
		r := iotest.ErrReader(errors.New("lemon"))
		_, err := cargo.InterpolateAndParseKilnfile(r, make(map[string]any))
		assert.Error(t, err)
	})
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

	please.Expect(err).To(MatchError(ContainSubstring(`could not find variable with key "region"`)))
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

func TestReadKilnfiles(t *testing.T) {
	t.Run("missing Kilnfile", func(t *testing.T) {
		kilnfilePath := filepath.Join(t.TempDir(), "Kilnfile")
		_, _, err := cargo.ReadKilnfileAndKilnfileLock(kilnfilePath)
		assert.Error(t, err)
	})
	t.Run("missing Kilnfile.lock", func(t *testing.T) {
		kilnfilePath := filepath.Join(t.TempDir(), "Kilnfile")
		_ = os.WriteFile(kilnfilePath, nil, 0o666)
		f, _ := os.Create(kilnfilePath)
		_ = f.Close()
		_, _, err := cargo.ReadKilnfileAndKilnfileLock(kilnfilePath)
		assert.Error(t, err)
	})
	t.Run("invalid spec yaml", func(t *testing.T) {
		kilnfilePath := filepath.Join(t.TempDir(), "Kilnfile")
		_ = os.WriteFile(kilnfilePath, []byte(`slug: {}`), 0o666) // will cause parse type error
		_, _, err := cargo.ReadKilnfileAndKilnfileLock(kilnfilePath)
		assert.Error(t, err)
	})
	t.Run("invalid lock yaml", func(t *testing.T) {
		kilnfilePath := filepath.Join(t.TempDir(), "Kilnfile")
		_ = os.WriteFile(kilnfilePath, []byte(``), 0o666)
		_ = os.WriteFile(kilnfilePath+".lock", []byte(`releases: 7`), 0o666) // will cause parse type error
		_, _, err := cargo.ReadKilnfileAndKilnfileLock(kilnfilePath)
		assert.Error(t, err)
	})
	t.Run("parse files", func(t *testing.T) {
		kilnfilePath := filepath.Join(t.TempDir(), "Kilnfile")
		_ = os.WriteFile(kilnfilePath, []byte(`slug: "banana"`), 0o666)
		_ = os.WriteFile(kilnfilePath+".lock", []byte(`releases: [{}]`), 0o666) // will cause parse type error
		spec, lock, err := cargo.ReadKilnfileAndKilnfileLock(kilnfilePath)
		assert.NoError(t, err)
		assert.Equal(t, "banana", spec.Slug)
		assert.Len(t, lock.Releases, 1)
	})
}

func TestWriteKilnfile(t *testing.T) {
	t.Run("it fails to write to a directory", func(t *testing.T) {
		// you should call ResolveKilnfilePath on the input
		dir := t.TempDir()
		assert.Error(t, cargo.WriteKilnfile(dir, cargo.Kilnfile{
			Slug: "banana",
		}))
	})
	t.Run("it writes a Kilnfile", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "Kilnfile")
		assert.NoError(t, cargo.WriteKilnfile(dir, cargo.Kilnfile{
			Slug: "banana",
		}))
		assert.FileExists(t, dir)

		kfYAML, err := os.ReadFile(dir)
		assert.NoError(t, err)
		assert.Contains(t, string(kfYAML), "slug: banana")
	})
}

func TestResolveKilnfilePath(t *testing.T) {
	t.Run("path to an existing Kilnfile", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "Kilnfile")
		require.NoError(t, os.WriteFile(path, nil, 0o666))
		result, err := cargo.ResolveKilnfilePath(path)
		assert.NoError(t, err)
		require.Equal(t, path, result)
	})
	t.Run("path is a directory", func(t *testing.T) {
		path := t.TempDir()
		result, err := cargo.ResolveKilnfilePath(path)
		assert.NoError(t, err)
		require.Equal(t, filepath.Join(path, "Kilnfile"), result)
	})
	t.Run("path to a non-directory file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "not-a-directory")
		require.NoError(t, os.WriteFile(path, nil, 0o666))
		_, err := cargo.ResolveKilnfilePath(path)
		assert.Error(t, err)
	})
	t.Run("path to a an invalid path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "not-a-directory")
		_, err := cargo.ResolveKilnfilePath(path)
		assert.Error(t, err)
	})
	t.Run("path to a lock", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "Kilnfile.lock")
		require.NoError(t, os.WriteFile(path, nil, 0o666))
		result, err := cargo.ResolveKilnfilePath(path)
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "Kilnfile"), result)
	})
}
