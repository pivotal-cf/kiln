package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_setKilnMetadata(t *testing.T) {
	for _, tt := range []struct{ Name string }{
		{Name: "append_kiln_metadata"},
		{Name: "replace_kiln_metadata"},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			inputMetadataYML, err := os.ReadFile(filepath.Join("testdata", tt.Name, "input_metadata.yml"))
			require.NoError(t, err)
			outputMetadataYML, err := os.ReadFile(filepath.Join("testdata", tt.Name, "output_metadata.yml"))
			require.NoError(t, err)

			result, err := setKilnMetadata(inputMetadataYML, KilnMetadata{
				MetadataGitSHA: "some-commit-sha",
			})
			require.NoError(t, err)
			assert.Equal(t, string(outputMetadataYML), string(result))
		})
	}
}

func Test_newKilnMetadata(t *testing.T) {
	t.Run("when tile name is set", func(t *testing.T) {
		input := InterpolateInput{
			MetadataGitSHA: "some-sha",
			Variables: map[string]any{
				TileNameVariable: "some-tile",
			},
		}

		km := newKilnMetadata(input)

		require.Equal(t, KilnMetadata{
			MetadataGitSHA: "some-sha",
			TileName:       "some-tile",
		}, km)
	})

	t.Run("when no tile name is set", func(t *testing.T) {
		input := InterpolateInput{
			MetadataGitSHA: "some-sha",
			Variables:      nil,
		}

		km := newKilnMetadata(input)

		require.Equal(t, KilnMetadata{
			MetadataGitSHA: "some-sha",
			TileName:       "",
		}, km)
	})

	t.Run("when no tile name is the wrong type", func(t *testing.T) {
		input := InterpolateInput{
			MetadataGitSHA: "some-sha",
			Variables: map[string]any{
				TileNameVariable: 11,
			},
		}

		km := newKilnMetadata(input)

		require.Equal(t, KilnMetadata{
			MetadataGitSHA: "some-sha",
			TileName:       "",
		}, km)
	})
}
