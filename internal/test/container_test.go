package test

import (
	"os"
	"path/filepath"
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
