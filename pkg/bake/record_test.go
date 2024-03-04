package bake_test

import (
	"github.com/pivotal-cf/kiln/pkg/bake"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"

	"github.com/pivotal-cf/kiln/internal/builder"

	"github.com/stretchr/testify/require"
)

func TestNewRecordFromFile(t *testing.T) {
	tilePath := filepath.Join("testdata", "tile.pivotal")
	record, err := bake.NewRecordFromFile(tilePath)
	require.NoError(t, err)
	assert.Equal(t, bake.Record{
		Version:        "0.2.0-dev",
		SourceRevision: "5874e0f81d0af47922716a7c69a08bcdead13348",
		KilnVersion:    "70092ce5ce6c0fd6b2434add95774e2612af5a51",
		FileChecksum:   "7490ba0b736c262ee7dc433c423c4f95ad838b014769d8465c50e445967d2735",
	}, record)
}

func TestNewRecord(t *testing.T) {
	t.Run("when creating a bake record from a product template", func(t *testing.T) {
		// language=yaml
		b, err := bake.NewRecord("some-peach-jam", []byte(`
product_name: p-each
product_version: some-product-version
kiln_metadata:
  kiln_version: some-kiln-version
  metadata_git_sha: some-tile-source-revision
  tile_name: srt
`))
		require.NoError(t, err)
		require.Equal(t, bake.Record{
			Version:        "some-product-version",
			KilnVersion:    "some-kiln-version",
			SourceRevision: "some-tile-source-revision",
			TileName:       "srt",
			FileChecksum:   "some-peach-jam",
		}, b)
	})

	t.Run("when the product template is missing kiln_metadata", func(t *testing.T) {
		// language=yaml
		_, err := bake.NewRecord("some-peach-jam", []byte(`
product_name: p-each
product_version: some-product-version
`))
		require.ErrorContains(t, err, "kiln_metadata")
	})

	t.Run("write one file", func(t *testing.T) {
		dir := t.TempDir()

		b := bake.Record{
			TileName:       "p-each",
			SourceRevision: "some-revision",
			Version:        "1.2.3",
			KilnVersion:    "some-version",
		}

		require.NoError(t, b.WriteFile(dir))

		buf, err := os.ReadFile(filepath.Join(dir, bake.RecordsDirectory, "p-each-1.2.3.json"))
		require.NoError(t, err)

		require.JSONEq(t, `{"source_revision":"some-revision", "tile_name":"p-each", "version":"1.2.3", "kiln_version": "some-version"}`, string(buf))
	})

	t.Run("when the record is missing the version field", func(t *testing.T) {
		dir := t.TempDir()

		b := bake.Record{
			Version: "",
		}

		require.ErrorContains(t, b.WriteFile(dir), "version")
	})

	t.Run("when a record is marked as developement", func(t *testing.T) {
		dir := t.TempDir()

		b := bake.Record{
			Version:        "1.2.3",
			SourceRevision: builder.DirtyWorktreeSHAValue,
		}

		require.ErrorContains(t, b.WriteFile(dir), "will not write development")
	})

	t.Run("write only required some fields", func(t *testing.T) {
		dir := t.TempDir()

		b := bake.Record{
			Version: "some-version",
		}

		require.NoError(t, b.WriteFile(dir))

		buf, err := os.ReadFile(filepath.Join(dir, bake.RecordsDirectory, "some-version.json"))
		require.NoError(t, err)

		require.JSONEq(t, `{"source_revision":"", "version":"some-version", "kiln_version": ""}`, string(buf))
	})

	t.Run("when a build record with the same version already exists", func(t *testing.T) {
		dir := t.TempDir()

		b := bake.Record{
			TileName: "some-tile",
			Version:  "some-version",
		}

		require.NoError(t, b.WriteFile(dir))
		require.ErrorContains(t, b.WriteFile(dir), "tile bake record already exists for some-tile/some-version")
	})

	t.Run("when read builds", func(t *testing.T) {
		dir := t.TempDir()

		bs := []bake.Record{
			{ // non standard semver
				TileName:       "p-each",
				SourceRevision: "some-hash-000",
				KilnVersion:    "some-kiln-version",
				Version:        "0.1.0.0",
				FileChecksum:   "some-hash-browns",
			},
			{
				TileName:       "p-each",
				SourceRevision: "some-hash-000",
				KilnVersion:    "some-kiln-version",
				Version:        "0.1.0.2",
				FileChecksum:   "some-hash-browns",
			},
			{
				TileName:       "p-each",
				SourceRevision: "some-hash-000",
				KilnVersion:    "some-kiln-version",
				Version:        "1.1.0",
				FileChecksum:   "some-hash-browns",
			},
			{
				TileName:       "p-each",
				SourceRevision: "some-hash-002",
				KilnVersion:    "some-kiln-version",
				Version:        "1.2.0",
				FileChecksum:   "some-hash-browns",
			},
			{
				TileName:       "p-each",
				SourceRevision: "some-hash-003",
				KilnVersion:    "some-kiln-version",
				Version:        "2.0.0",
				FileChecksum:   "some-hash-browns",
			},
			{
				TileName:       "p-ear",
				SourceRevision: "some-hash-004",
				KilnVersion:    "some-kiln-version",
				Version:        "2.0.0",
				FileChecksum:   "some-hash-browns",
			},
			{
				TileName:       "p-each",
				SourceRevision: "some-hash-005",
				KilnVersion:    "some-kiln-version",
				Version:        "2.2.0",
				FileChecksum:   "some-hash-browns",
			},
		}

		for _, b := range bs {
			require.NoError(t, b.WriteFile(dir))
		}

		result, err := bake.ReadRecords(os.DirFS(dir))
		require.NoError(t, err)

		require.Equal(t, bs, result, "the builds are in order and contain all the info")
	})
}
