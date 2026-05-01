package test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func TestConfiguration_commands(t *testing.T) {
	absoluteTileDirectory := filepath.Join(t.TempDir(), "test")
	require.NoError(t, os.MkdirAll(absoluteTileDirectory, 0o700))

	for _, tt := range []struct {
		Name            string
		Configuration   Configuration
		Result          []string
		ExpErrSubstring string
	}{
		{
			Name: "when the tile path is not absolute",
			Configuration: Configuration{
				AbsoluteTileDirectory: ".",
			},
			ExpErrSubstring: "tile path must be absolute",
		},
		{
			Name: "when no flags are true",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
			},
			Result: []string{"git config --global --add safe.directory '*'"},
		},
		{
			Name: "when running migrations tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunMigrations:         true,
			},
			Result: []string{"git config --global --add safe.directory '*'", "cd /tas/test/migrations", "npm install", "npm test"},
		},
		{
			Name: "when running manifest tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunManifest:           true,
			},
			Result: []string{"git config --global --add safe.directory '*'", "cd /tas/test && ginkgo  /tas/test/test/manifest"},
		},
		{
			Name: "when running metadata tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunMetadata:           true,
			},
			Result: []string{"git config --global --add safe.directory '*'", "cd /tas/test && ginkgo  /tas/test/test/stability"},
		},
		{
			Name: "when running all tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunAll:                true,
			},
			Result: []string{"git config --global --add safe.directory '*'", "cd /tas/test/migrations", "npm install", "npm test", "cd /tas/test && ginkgo  /tas/test/test/stability /tas/test/test/manifest"},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			result, err := tt.Configuration.commands()
			if tt.ExpErrSubstring != "" {
				require.ErrorContains(t, err, tt.ExpErrSubstring)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.Result, result)
			}
		})
	}
}

func Test_checkImageBuildResponse(t *testing.T) {
	t.Run("streams build log then error", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader(
			`{"stream":"Step 1\n"}` + "\n" +
				`{"stream":"go: downloading\n"}` + "\n" +
				`{"error":"failed","errorDetail":{"message":"go install: nope"}}` + "\n",
		))
		var buf bytes.Buffer
		err := checkImageBuildResponse(body, &buf)
		require.ErrorContains(t, err, "go install: nope")
		require.Contains(t, buf.String(), "Step 1")
		require.Contains(t, buf.String(), "go: downloading")
	})
}

func TestEmbeddedDockerfile_usesDockerVirtualRegistry(t *testing.T) {
	require.Contains(t, dockerfile, "FROM "+DockerVirtualRegistryHost+"/golang")
	require.Contains(t, dockerfile, "FROM "+DockerVirtualRegistryHost+"/ruby:3.4.8")
	require.NotContains(t, dockerfile, "REGISTRY_PREFIX")
	require.Contains(t, dockerfile, "ENV GOPROXY=https://${ARTIFACTORY_USERNAME}:${ARTIFACTORY_PASSWORD}@usw1.packages.broadcom.com/artifactory/api/go/tas-rel-eng-go-virtual")
	require.Contains(t, dockerfile, "ENV GOSUMDB=off")
}

func Test_registryAuthForDockerVirtual(t *testing.T) {
	t.Run("nil when username missing", func(t *testing.T) {
		require.Nil(t, registryAuthForDockerVirtual(environmentVars{"ARTIFACTORY_PASSWORD": "p"}))
	})
	t.Run("nil when password missing", func(t *testing.T) {
		require.Nil(t, registryAuthForDockerVirtual(environmentVars{"ARTIFACTORY_USERNAME": "u"}))
	})
	t.Run("returns auth for docker virtual host", func(t *testing.T) {
		got := registryAuthForDockerVirtual(environmentVars{
			"ARTIFACTORY_USERNAME": "alice",
			"ARTIFACTORY_PASSWORD": "secret",
		})
		require.Len(t, got, 1)
		cfg := got[DockerVirtualRegistryHost]
		require.Equal(t, "alice", cfg.Username)
		require.Equal(t, "secret", cfg.Password)
		require.Equal(t, DockerVirtualRegistryHost, cfg.ServerAddress)
	})
}

func Test_RequiredArtifactoryCredentials(t *testing.T) {
	t.Run("from -e only", func(t *testing.T) {
		t.Setenv("ARTIFACTORY_USERNAME", "")
		t.Setenv("ARTIFACTORY_PASSWORD", "")
		u, p, err := RequiredArtifactoryCredentials([]string{"ARTIFACTORY_USERNAME=a", "ARTIFACTORY_PASSWORD=b"})
		require.NoError(t, err)
		require.Equal(t, "a", u)
		require.Equal(t, "b", p)
	})
	t.Run("-e overrides process env", func(t *testing.T) {
		t.Setenv("ARTIFACTORY_USERNAME", "envuser")
		t.Setenv("ARTIFACTORY_PASSWORD", "envpass")
		u, p, err := RequiredArtifactoryCredentials([]string{"ARTIFACTORY_USERNAME=fromflag", "ARTIFACTORY_PASSWORD=frompass"})
		require.NoError(t, err)
		require.Equal(t, "fromflag", u)
		require.Equal(t, "frompass", p)
	})
	t.Run("missing username", func(t *testing.T) {
		t.Setenv("ARTIFACTORY_USERNAME", "")
		_, _, err := RequiredArtifactoryCredentials([]string{"ARTIFACTORY_PASSWORD=only"})
		require.ErrorContains(t, err, "ARTIFACTORY_USERNAME")
		require.ErrorContains(t, err, "kiln test")
	})
	t.Run("missing password", func(t *testing.T) {
		t.Setenv("ARTIFACTORY_PASSWORD", "")
		_, _, err := RequiredArtifactoryCredentials([]string{"ARTIFACTORY_USERNAME=only"})
		require.ErrorContains(t, err, "ARTIFACTORY_PASSWORD")
		require.ErrorContains(t, err, "kiln test")
	})
	t.Run("invalid env pair", func(t *testing.T) {
		_, _, err := RequiredArtifactoryCredentials([]string{"notakeyval"})
		require.Error(t, err)
	})
}

func Test_decodeEnvironment(t *testing.T) {
	for _, tt := range []struct {
		Name            string
		In              []string
		Exp             map[string]string
		ExpErrSubstring string
	}{
		{
			Name: "valid variable",
			In:   []string{"fruit=orange"},
			Exp: map[string]string{
				"fruit": "orange",
			},
		},
		{
			Name:            "no separator",
			In:              []string{"fruit:orange"},
			ExpErrSubstring: "environment variables must have the format [key]=[value]",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := decodeEnvironment(tt.In)
			if tt.ExpErrSubstring != "" {
				require.ErrorContains(t, err, tt.ExpErrSubstring)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.Exp, got)
			}
		})
	}
}
