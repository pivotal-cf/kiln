package gh_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pivotal-cf/kiln/internal/gh"
)

func TestClient(t *testing.T) {
	ctx := context.Background()
	token := "xxx"
	ghClient := gh.Client(ctx, token)
	require.NotNil(t, ghClient.Client())
}
