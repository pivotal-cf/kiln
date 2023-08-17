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

func TestNonStandardMetadataFilename(t *testing.T) {
	fileFS := fstest.MapFS{
		"metadata/banana.yml": &fstest.MapFile{Data: []byte(`{name: "banana"}`)},
	}
	buf, err := tile.ReadMetadataFromFS(fileFS)
	require.NoError(t, err)

	assert.Equal(t, string(buf), "{name: \"banana\"}")
}

func TestReadMetadataFromProductFile(t *testing.T) {
	var authHeader string
	client, downloadLink, authorizationHeader := setupReadMetadataFromProductFile(t, &authHeader)
	ctx := context.Background()

	metadataBytes, err := tile.ReadMetadataFromProductFile(ctx, client, downloadLink, authorizationHeader)
	require.NoError(t, err)
	assert.Equal(t, authHeader, "Token some-token")

	var metadata struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(metadataBytes, &metadata)
	require.NoError(t, err)

	assert.Equal(t, "hello", metadata.Name)
}

func setupReadMetadataFromProductFile(t *testing.T, authHeaderFromRequest *string) (*http.Client, string, string) {
	t.Helper()
	fileServer := http.FileServer(http.Dir("testdata"))
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		*authHeaderFromRequest = req.Header.Get("authorization")
		fileServer.ServeHTTP(res, req)
	}))
	t.Cleanup(server.Close)

	//// This is rough untested example code for how one might configure a real pivnet client.
	// config := pivnet.ClientConfig{
	//	Host:      pivnet.DefaultHost,
	//	UserAgent: "kiln-test",
	// }
	// pivnetToken := os.Getenv("PIVNET_TOKEN")
	// tanzuNetClient := pivnet.NewAccessTokenOrLegacyToken(pivnetToken, server.URL, config.SkipSSLValidation)
	// accessToken, err := tanzuNetClient.AccessToken()
	// require.NoError(err)
	accessToken := "some-token"
	authorizationHeader, err := pivnet.AuthorizationHeader(accessToken)
	require.NoError(t, err)

	// productFile should come from an API request:
	// _ = pivnet.ProductFilesService.ListForRelease
	productFile := pivnet.ProductFile{
		Links: &pivnet.Links{
			Download: map[string]string{"href": server.URL + "/tile-0.1.2.pivotal"},
		},
	}

	downloadLink, err := productFile.DownloadLink()
	require.NoError(t, err)

	return server.Client(), downloadLink, authorizationHeader
}
