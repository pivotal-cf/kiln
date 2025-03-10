package tile_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/pivotal-cf/go-pivnet/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/tile"
)

func TestReadMetadataFromFile(t *testing.T) {
	metadataBytes, err := tile.ReadMetadataFromFile("testdata/tile-0.1.2.pivotal")
	require.NoError(t, err)

	var metadata struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(metadataBytes, &metadata)
	require.NoError(t, err)

	assert.Equal(t, metadata.Name, "hello")
}
func TestReadMetadataFromFile_NonExistingFile(t *testing.T) {
	_, err := tile.ReadMetadataFromFile("testdata/no-tile.pivotal")
	require.Error(t, err)
}

func TestNonStandardMetadataFilename(t *testing.T) {
	fileFS := fstest.MapFS{
		"metadata/banana.yml": &fstest.MapFile{Data: []byte(`{name: "banana"}`)},
	}
	buf, err := tile.ReadMetadataFromFS(fileFS)
	require.NoError(t, err)

	assert.Equal(t, string(buf), "{name: \"banana\"}")
}

func TestReadMetadataFromFS_NonExistingMetadataFile(t *testing.T) {
	fileFS := fstest.MapFS{
		"metadata/banana.json": &fstest.MapFile{Data: []byte(`{name: "banana"}`)},
	}
	_, err := tile.ReadMetadataFromFS(fileFS)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata file not found in the tile: expected a file matching glob")
}

func TestReadMetadataFromServer(t *testing.T) {
	httpClient, productFile := setupReadMetadataFromServer(t)

	// create http.Request from pivnet.ProductFile
	downloadLink, err := productFile.DownloadLink()
	require.NoError(t, err)
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadLink, nil)
	require.NoError(t, err)
	// on a real request you need to set Authorization and User-Agent headers

	metadataBytes, err := tile.ReadMetadataFromServer(httpClient, req)
	require.NoError(t, err)

	var metadata struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(metadataBytes, &metadata)
	require.NoError(t, err)

	assert.Equal(t, "hello", metadata.Name)
}

func setupReadMetadataFromServer(t *testing.T) (*http.Client, pivnet.ProductFile) {
	t.Helper()
	fileServer := http.FileServer(http.Dir("testdata"))
	server := httptest.NewServer(fileServer)
	t.Cleanup(server.Close)

	// productFile should come from an API request:
	// _ = pivnet.ProductFilesService.ListForRelease
	productFile := pivnet.ProductFile{
		Links: &pivnet.Links{
			Download: map[string]string{"href": server.URL + "/tile-0.1.2.pivotal"},
		},
	}

	return server.Client(), productFile
}
