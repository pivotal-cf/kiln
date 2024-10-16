package gh_test

import (
	"context"
	"testing"

	"github.com/pivotal-cf/kiln/internal/gh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Run("when the host is empty", func(t *testing.T) {
		ctx := context.Background()
		token := "xxx"
		ghClient, err := gh.Client(ctx, "", token, token)
		require.NoError(t, err)
		require.NotNil(t, ghClient.Client())
		assert.Contains(t, ghClient.BaseURL.String(), "https://api.github.com")
	})

	t.Run("when the host point to enterprise github", func(t *testing.T) {
		ctx := context.Background()
		token := "xxx"
		ghClient, err := gh.Client(ctx, "https://broadcom.net", token, token)
		require.NoError(t, err)
		require.NotNil(t, ghClient.Client())
		assert.Contains(t, ghClient.BaseURL.String(), "https://broadcom.net")
	})

	t.Run("when the host point to non-enterprise random github", func(t *testing.T) {
		ctx := context.Background()
		token := "xxx"
		ghClient, err := gh.Client(ctx, "https://example.com", token, token)
		require.Error(t, err, "github host not recognized")
		require.Nil(t, ghClient)
	})
}
