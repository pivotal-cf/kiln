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
		ExpPlan         testPlan
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
			ExpPlan: testPlan{
				setup: []string{"git config --global --add safe.directory '*'"},
			},
		},
		{
			Name: "when running migrations tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunMigrations:         true,
			},
			ExpPlan: testPlan{
				setup: []string{"git config --global --add safe.directory '*'"},
				suites: []suiteStep{
					{
						name: "Migration Tests",
						cmds: []string{
							"cd /tas/test/migrations",
							"npm install --no-audit --no-fund --silent",
							`printf '\nRunning Suite: Migration Tests\n==============================\n'`,
							"npm test",
						},
					},
				},
			},
		},
		{
			Name: "when running manifest tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunManifest:           true,
			},
			ExpPlan: testPlan{
				setup: []string{"git config --global --add safe.directory '*'"},
				suites: []suiteStep{
					{
						name: "Manifest Tests",
						cmds: []string{
							`printf '\n'`,
							"cd /tas/test && ginkgo  /tas/test/test/manifest",
						},
					},
				},
			},
		},
		{
			Name: "when running metadata tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunMetadata:           true,
			},
			ExpPlan: testPlan{
				setup: []string{"git config --global --add safe.directory '*'"},
				suites: []suiteStep{
					{
						name: "Stability Tests",
						cmds: []string{
							`printf '\nRunning Suite: Stability Tests\n==============================\n'`,
							"cd /tas/test && ginkgo  /tas/test/test/stability",
						},
					},
				},
			},
		},
		{
			Name: "when running all tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunAll:                true,
			},
			ExpPlan: testPlan{
				setup: []string{"git config --global --add safe.directory '*'"},
				suites: []suiteStep{
					{
						name: "Migration Tests",
						cmds: []string{
							"cd /tas/test/migrations",
							"npm install --no-audit --no-fund --silent",
							`printf '\nRunning Suite: Migration Tests\n==============================\n'`,
							"npm test",
						},
					},
					{
						name: "Stability Tests",
						cmds: []string{
							`printf '\nRunning Suite: Stability Tests\n==============================\n'`,
							"cd /tas/test && ginkgo  /tas/test/test/stability",
						},
					},
					{
						name: "Manifest Tests",
						cmds: []string{
							`printf '\n'`,
							"cd /tas/test && ginkgo  /tas/test/test/manifest",
						},
					},
				},
			},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			plan, err := tt.Configuration.commands()
			if tt.ExpErrSubstring != "" {
				require.ErrorContains(t, err, tt.ExpErrSubstring)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.ExpPlan.setup, plan.setup)
			require.Len(t, plan.suites, len(tt.ExpPlan.suites))
			for i, expSuite := range tt.ExpPlan.suites {
				require.Equal(t, expSuite.name, plan.suites[i].name)
				require.Equal(t, expSuite.cmds, plan.suites[i].cmds)
			}
		})
	}
}

func TestTestPlan_script_includesSummaryForMultipleSuites(t *testing.T) {
	plan := testPlan{
		setup: []string{"setup cmd"},
		suites: []suiteStep{
			{name: "Migration Tests", cmds: []string{"npm test"}},
			{name: "Stability Tests", cmds: []string{"ginkgo stability"}},
		},
	}

	script := plan.script()

	// Each suite runs in a subshell with captured exit code.
	require.Contains(t, script, "); _exit0=$?")
	require.Contains(t, script, "); _exit1=$?")

	// End time captured right after each suite.
	require.Contains(t, script, "_time0=$(date")
	require.Contains(t, script, "_time1=$(date")

	// Summary lines present for both suites with captured timestamps.
	require.Contains(t, script, "Migration Tests Passed")
	require.Contains(t, script, "Migration Tests Failed")
	require.Contains(t, script, "Stability Tests Passed")
	require.Contains(t, script, "Stability Tests Failed")
	require.Contains(t, script, "$_time0")
	require.Contains(t, script, "$_time1")

	// ANSI green and red codes present.
	require.Contains(t, script, "\\033[32m")
	require.Contains(t, script, "\\033[31m")

	// Pass/fail symbols present.
	require.Contains(t, script, "✓")
	require.Contains(t, script, "✗")

	// Overall exit present.
	require.Contains(t, script, "_overall")
	require.Contains(t, script, "exit $_overall")
}

func TestTestPlan_script_omitsSummaryForSingleSuite(t *testing.T) {
	plan := testPlan{
		setup: []string{"setup cmd"},
		suites: []suiteStep{
			{name: "Manifest Tests", cmds: []string{"ginkgo manifest"}},
		},
	}

	script := plan.script()

	// No summary text for single suite.
	require.NotContains(t, script, "Passed")
	require.NotContains(t, script, "Failed")

	// Still exits with the suite's exit code.
	require.Contains(t, script, "exit $_overall")
}

func TestTestPlan_script_emptyWithNoSuites(t *testing.T) {
	plan := testPlan{
		setup: []string{"git config --global --add safe.directory '*'"},
	}
	script := plan.script()
	require.Contains(t, script, "git config")
	require.NotContains(t, script, "_exit0")
	require.NotContains(t, script, "_overall")
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

func TestEmbeddedDockerfile_structure(t *testing.T) {
	// Base image FROM lines must use the internal docker-virtual registry.
	require.Contains(t, dockerfile, "FROM "+DockerVirtualRegistryHost+"/golang")
	require.Contains(t, dockerfile, "FROM "+DockerVirtualRegistryHost+"/ruby:3.4.8")
	require.NotContains(t, dockerfile, "REGISTRY_PREFIX")

	// ginkgo must be pinned to a specific version (not @latest) so builds are reproducible
	// and the cache layer is stable.
	require.Contains(t, dockerfile, "go install github.com/onsi/ginkgo/ginkgo@v1.16.5")
	require.NotContains(t, dockerfile, "ginkgo@latest")

	// GOPROXY credentials must be scoped to the ginkgo RUN step only — not exported
	// as an ENV layer — so the ginkgo install layer is not busted by credential rotation.
	require.NotContains(t, dockerfile, "ENV GOPROXY=https://${ARTIFACTORY_USERNAME}")

	// Credentials ARG declaration must come AFTER stable system package installs
	// (jq, nodejs, npm) so those layers stay cached when credentials rotate.
	argIdx := strings.Index(dockerfile, "ARG ARTIFACTORY_USERNAME")
	jqIdx := strings.Index(dockerfile, "apt-get")
	require.Greater(t, argIdx, jqIdx, "ARTIFACTORY_USERNAME ARG should appear after apt-get installs")

	// Credentials must be exported to ENV for ops-manifest gem at container runtime.
	require.Contains(t, dockerfile, "ENV ARTIFACTORY_USERNAME=${ARTIFACTORY_USERNAME}")
	require.Contains(t, dockerfile, "ENV ARTIFACTORY_PASSWORD=${ARTIFACTORY_PASSWORD}")
}

func TestConfiguration_commands_usesNpmCiWhenLockfilePresent(t *testing.T) {
	tileDir := filepath.Join(t.TempDir(), "ist")
	require.NoError(t, os.MkdirAll(filepath.Join(tileDir, "migrations"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tileDir, "migrations", "package-lock.json"), []byte("{}"), 0o600))

	plan, err := Configuration{AbsoluteTileDirectory: tileDir, RunMigrations: true}.commands()
	require.NoError(t, err)
	require.Len(t, plan.suites, 1)
	// verbose=false (default): npm output silenced
	require.Contains(t, plan.suites[0].cmds, "npm ci --silent")
}

func TestConfiguration_commands_verboseUsesNpmCiWithoutSilent(t *testing.T) {
	tileDir := filepath.Join(t.TempDir(), "ist")
	require.NoError(t, os.MkdirAll(filepath.Join(tileDir, "migrations"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tileDir, "migrations", "package-lock.json"), []byte("{}"), 0o600))

	plan, err := Configuration{AbsoluteTileDirectory: tileDir, RunMigrations: true, Verbose: true}.commands()
	require.NoError(t, err)
	require.Contains(t, plan.suites[0].cmds, "npm ci")
	require.NotContains(t, plan.suites[0].cmds, "npm ci --silent")
}

func TestConfiguration_commands_usesNpmInstallWithoutLockfile(t *testing.T) {
	tileDir := filepath.Join(t.TempDir(), "ist")
	require.NoError(t, os.MkdirAll(filepath.Join(tileDir, "migrations"), 0o700))

	plan, err := Configuration{AbsoluteTileDirectory: tileDir, RunMigrations: true}.commands()
	require.NoError(t, err)
	require.Contains(t, plan.suites[0].cmds, "npm install --no-audit --no-fund --silent")
}

func TestTestPlan_script_verbose_addsStartAndEndTimestamps(t *testing.T) {
	plan := testPlan{
		setup:   []string{"setup cmd"},
		verbose: true,
		suites: []suiteStep{
			{name: "Migration Tests", cmds: []string{"npm test"}},
			{name: "Stability Tests", cmds: []string{"ginkgo stability"}},
		},
	}

	script := plan.script()

	// Start and end echo lines present for each suite.
	require.Contains(t, script, "Starting: Migration Tests")
	require.Contains(t, script, "Completed: Migration Tests")
	require.Contains(t, script, "Starting: Stability Tests")
	require.Contains(t, script, "Completed: Stability Tests")
}

func TestTestPlan_script_noStartEndEchoWhenNotVerbose(t *testing.T) {
	plan := testPlan{
		setup:   []string{"setup cmd"},
		verbose: false,
		suites:  []suiteStep{{name: "Migration Tests", cmds: []string{"npm test"}}},
	}

	script := plan.script()

	// No verbose echo lines; end-time variable is still captured for potential summary use.
	require.NotContains(t, script, "Starting:")
	require.NotContains(t, script, "Completed:")
	require.Contains(t, script, "_time0=$(date")
}

func TestGetTileTestEnvVars_setsGOMAXPROCS(t *testing.T) {
	tileDir := filepath.Join(t.TempDir(), "ist")
	envVars := getTileTestEnvVars(tileDir, "ist", environmentVars{})
	gomaxprocs, ok := envVars["GOMAXPROCS"]
	require.True(t, ok, "GOMAXPROCS should be set in container env")
	require.NotEmpty(t, gomaxprocs)
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
