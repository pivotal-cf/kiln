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
				KilnVersion:    "some-kiln-version",
			})
			require.NoError(t, err)
			assert.Equal(t, string(outputMetadataYML), string(result))
		})
	}
}
