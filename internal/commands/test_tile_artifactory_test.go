package commands

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pivotal-cf/kiln/internal/test"
)

func TestTileTest_RequiresArtifactoryCredentials(t *testing.T) {
	t.Setenv("ARTIFACTORY_USERNAME", "")
	t.Setenv("ARTIFACTORY_PASSWORD", "")

	err := NewTileTest().Execute([]string{})
	require.Error(t, err)
	require.ErrorContains(t, err, "ARTIFACTORY_USERNAME")
	require.ErrorContains(t, err, "kiln test")
}

func TestTileTest_RequiresArtifactoryPassword(t *testing.T) {
	t.Setenv("ARTIFACTORY_PASSWORD", "")

	err := NewTileTest().Execute([]string{"-e", "ARTIFACTORY_USERNAME=onlyuser"})
	require.Error(t, err)
	require.ErrorContains(t, err, "ARTIFACTORY_PASSWORD")
}

func TestTileTest_PassesArtifactoryViaEnvironmentToConfiguration(t *testing.T) {
	var captured test.Configuration
	stub := func(_ context.Context, _ io.Writer, c test.Configuration) error {
		captured = c
		return nil
	}
	err := NewTileTestWithCollaborators(io.Discard, stub).Execute([]string{
		"-e", "ARTIFACTORY_USERNAME=u",
		"-e", "ARTIFACTORY_PASSWORD=p",
	})
	require.NoError(t, err)
	require.Contains(t, captured.Environment, "ARTIFACTORY_USERNAME=u")
	require.Contains(t, captured.Environment, "ARTIFACTORY_PASSWORD=p")
}

func TestTileTest_UsesProcessEnvArtifactoryCredentials(t *testing.T) {
	t.Setenv("ARTIFACTORY_USERNAME", "fromenv")
	t.Setenv("ARTIFACTORY_PASSWORD", "frompass")

	var captured test.Configuration
	stub := func(_ context.Context, _ io.Writer, c test.Configuration) error {
		captured = c
		return nil
	}
	err := NewTileTestWithCollaborators(io.Discard, stub).Execute([]string{})
	require.NoError(t, err)
	require.Empty(t, captured.Environment)
}
