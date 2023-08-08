//go:build acceptance

// Package workflows executes cucumber style acceptance tests.
//
// To run the tests execute:
//
//	go test -v --tags acceptance --timeout=1h github.com/pivotal-cf/kiln/internal/acceptance/workflows
//
// To run a particular test execute (notice the run tag value is a case-sensitive regular expression):
//
//	go test --run baking -v --tags acceptance --timeout=1h github.com/pivotal-cf/kiln/internal/acceptance/workflows
package workflows

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"github.com/pivotal-cf/kiln/internal/acceptance/workflows/scenario"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func Test_baking_a_tile(t *testing.T) {
	// t.SkipNow()
	setupAndRunFeatureTest(t)
}

func Test_caching_compiled_releases(t *testing.T) {
	setupAndRunFeatureTest(t,
		scenario.InitializeCacheCompiledReleases,
	)
}

func Test_generating_release_notes(t *testing.T) {
	setupAndRunFeatureTest(t, scenario.InitializeGitHub)
}

func Test_updating_releases(t *testing.T) {
	setupAndRunFeatureTest(t,
		scenario.InitializeGitHub,
	)
}

func Test_glaze(t *testing.T) {
	setupAndRunFeatureTest(t)
}

func Test_updating_stemcell(t *testing.T) {
	setupAndRunFeatureTest(t,
		scenario.InitializeTanzuNetwork,
	)
}

func Test_using_kiln(t *testing.T) {
	setupAndRunFeatureTest(t)
}

func setupAndRunFeatureTest(t *testing.T, initializers ...func(ctx *godog.ScenarioContext)) {
	kilnExecutable := filepath.Join(t.TempDir(), "kiln")

	buildKilnBinary(t, kilnExecutable)

	trimmedTestFuncName := strings.TrimPrefix(t.Name(), "Test_")
	featurePath := trimmedTestFuncName + ".feature"

	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			// default initializers
			scenario.InitializeEnv(ctx)
			scenario.InitializeAWS(ctx)
			scenario.InitializeKiln(ctx)
			scenario.InitializeExec(ctx)
			scenario.InitializeRegex(ctx)
			scenario.InitializeTileSourceCode(ctx)
			scenario.InitializeTile(ctx)

			// additional initializers
			for _, initializer := range initializers {
				initializer(ctx)
			}
		},
		Options: &godog.Options{
			Format:         "pretty",
			Paths:          []string{featurePath},
			DefaultContext: scenario.DefaultContext(context.Background(), kilnExecutable),
			TestingT:       t, // Testing instance that will run subtests.
		},
	}

	if code := suite.Run(); code != 0 {
		t.Fatalf("status %d returned, failed to run %s", code, featurePath)
	}
}

func buildKilnBinary(t *testing.T, kilnExecutable string) {
	t.Helper()
	const kilnVersion = "0.0.0+acceptance-tests"
	goBuild := exec.Command("go", "build", "-o", kilnExecutable, "-ldflags", "-X main.version="+kilnVersion, "github.com/pivotal-cf/kiln")
	if testing.Verbose() {
		goBuild.Stderr = os.Stdout
		goBuild.Stdout = os.Stdout
		goBuild.Args = slices.Insert(goBuild.Args, slices.Index(goBuild.Args, "build")+1, "-v")
	}
	if err := goBuild.Run(); err != nil {
		t.Fatal(err)
	}
}
