package test_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	artifactoryUsername, usernameFound := os.LookupEnv("ARTIFACTORY_USERNAME")
	if !usernameFound {
		t.Fatal("Missing ARTIFACTORY_USERNAME environment variable")
	}

	artifactoryPassword, passwordFound := os.LookupEnv("ARTIFACTORY_PASSWORD")
	if !passwordFound {
		t.Fatal("Missing ARTIFACTORY_PASSWORD environment variable")
	}

	t.Run("the test succeeds", func(t *testing.T) {
		tmpDir := setupTestRepo(t)

		ctx := context.Background()
		configuration := test.Configuration{
			AbsoluteTileDirectory: tmpDir,
			RunAll:                true,
			Environment:           []string{"ARTIFACTORY_USERNAME=" + artifactoryUsername, "ARTIFACTORY_PASSWORD=" + artifactoryPassword},
		}
		out := io.Discard
		if testing.Verbose() {
			out = os.Stdout
		}

		err := test.Run(ctx, out, configuration)
		assert.NoError(t, err)
	})

	t.Run("the test fails", func(t *testing.T) {
		tmpDir := setupTestRepo(t)

		ctx := context.Background()
		configuration := test.Configuration{
			AbsoluteTileDirectory: tmpDir,
			RunManifest:           true,
			Environment:           []string{"FAIL_TEST=true", "ARTIFACTORY_USERNAME=" + artifactoryUsername, "ARTIFACTORY_PASSWORD=" + artifactoryPassword},
		}

		outBuffer := new(bytes.Buffer)
		var out io.Writer = outBuffer
		if testing.Verbose() {
			out = io.MultiWriter(outBuffer, os.Stdout)
		}

		err := test.Run(ctx, out, configuration)
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

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	ctx := context.Background()

	info, err := dockerClient.ServerVersion(ctx)
	require.NoError(t, err)

	minimumServerVersion := getConstraint(t, info.Components)
	require.NoError(t, err)

	serverVersion := semver.MustParse(info.Version)
	ok, reasons := minimumServerVersion.Validate(serverVersion)
	for _, reason := range reasons {
		t.Log(reason.Error())
	}
	require.True(t, ok, "kiln test requires a newer version of Docker/Podman")
}

func getConstraint(t *testing.T, components []types.ComponentVersion) *semver.Constraints {
	version := test.MinimumDockerServerVersion
	for _, component := range components {
		if component.Name == "Podman Engine" {
			version = test.MinimumPodmanServerVersion
		}
	}
	constraints, err := semver.NewConstraint(version)
	require.NoError(t, err)
	return constraints
}

func setupTestRepo(t *testing.T) string {
	wd, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	happyTilePath := filepath.Join(wd, "testdata", "happy-tile")
	tar, err := archive.Tar(happyTilePath, compression.None)
	assert.NoError(t, err)
	err = archive.Untar(tar, tmpDir, nil)
	assert.NoError(t, err)
	t.Cleanup(func() {
		err = os.RemoveAll(tmpDir)
		assert.NoError(t, err)
		_ = tar.Close()
	})

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "test@example.com"},
		{"git", "add", "."},
		{"git", "commit", "-m", "init"},
	}
	for _, cmdSlice := range cmds {
		cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
		cmd.Dir = tmpDir
		_ = cmd.Run()
	}

	return tmpDir
}
