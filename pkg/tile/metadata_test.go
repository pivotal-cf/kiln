package tile_test

import (
	"testing"
	"testing/fstest"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/tile"
	"github.com/stretchr/testify/require"
)

func TestReadMetadataFromFile(t *testing.T) {
	metadataBytes, err := tile.ReadMetadataFromFile("testdata/tile-0.1.2.pivotal")
	require.NoError(t, err)

	var metadata struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(metadataBytes, &metadata)
	require.NoError(t, err)

	require.Equal(t, "hello", metadata.Name)
}

func TestNonStandardMetadataFilename(t *testing.T) {
	fileFS := fstest.MapFS{
		"metadata/banana.yml": &fstest.MapFile{Data: []byte(`{name: "banana"}`)},
	}
	buf, err := tile.ReadMetadataFromFS(fileFS)
	require.NoError(t, err)
	require.Equal(t, `{name: "banana"}`, string(buf))
}
