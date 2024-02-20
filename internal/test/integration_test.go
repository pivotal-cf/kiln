package test_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Masterminds/semver/v3"
	"github.com/docker/docker/client"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/test"
)

var _ commands.TileTestFunction = test.Run

func TestDockerIntegration(t *testing.T) {
	_, githubWorkspaceEnvVarFound := os.LookupEnv("GITHUB_WORKSPACE")
	if testing.Short() || githubWorkspaceEnvVarFound {
		t.Skip("integration test is slow")
	}

	checkDaemonVersion(t)

	t.Run("the test succeeds", func(t *testing.T) {
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
	})

	t.Run("the test fails", func(t *testing.T) {
		wd, err := os.Getwd()
		require.NoError(t, err)

		ctx := context.Background()
		configuration := test.Configuration{
			AbsoluteTileDirectory: filepath.Join(wd, "testdata", "happy-tile"),
			RunManifest:           true,
			Environment:           []string{"FAIL_TEST=true"},
		}

		outBuffer := new(bytes.Buffer)
		var out io.Writer = outBuffer
		if testing.Verbose() {
			out = io.MultiWriter(outBuffer, os.Stdout)
		}

		err = test.Run(ctx, out, configuration)
		assert.Error(t, err)
		assert.Contains(t, outBuffer.String(), "FAIL_TEST is true")
	})
}

func checkDaemonVersion(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip()
	}

	_, githubWorkspaceEnvVarFound := os.LookupEnv("GITHUB_WORKSPACE")
	if testing.Short() || githubWorkspaceEnvVarFound {
		t.Skip("integration test is slow")
	}

	constraints, err := semver.NewConstraint(test.MinimumDockerServerVersion)
	require.NoError(t, err)

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	ctx := context.Background()

	info, err := dockerClient.ServerVersion(ctx)
	require.NoError(t, err)

	v := semver.MustParse(info.Version)

	ok, reasons := constraints.Validate(v)
	for _, reason := range reasons {
		t.Log(reason.Error())
	}
	require.True(t, ok, "kiln test requires a newer version of Docker")
}
