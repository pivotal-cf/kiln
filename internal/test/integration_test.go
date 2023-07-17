package test_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/test"
)

var _ commands.TileTestFunction = test.Run

func TestDockerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test is slow")
	}

	wd, err := os.Getwd()
	require.NoError(t, err)

	ctx := context.Background()
	configuration := test.Configuration{
		AbsoluteTileDirectory: filepath.Join(wd, "testdata", "happy-tile"),
		RunAll:                true,
	}
	out := io.Discard
	if testing.Verbose() {
		out = os.Stdout
	}

	err = test.Run(ctx, out, configuration)
	assert.NoError(t, err)
}
