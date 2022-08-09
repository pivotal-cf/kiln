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
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"github.com/pivotal-cf/kiln/internal/acceptance/workflows/scenario"
)

func Test_baking_a_tile(t *testing.T) {
	// t.SkipNow()
	setupAndRunFeatureTest(t)
}

func Test_updating_releases(t *testing.T) {
	setupAndRunFeatureTest(t,
		scenario.InitializeGitHub,
	)
}

func Test_caching_compiled_releases(t *testing.T) {
	setupAndRunFeatureTest(t,
		scenario.InitializeCacheCompiledReleases,
	)
}

func Test_generating_release_notes(t *testing.T) {
	setupAndRunFeatureTest(t, scenario.InitializeGitHub)
}

func setupAndRunFeatureTest(t *testing.T, initializers ...func(ctx *godog.ScenarioContext)) {
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
			Format:   "pretty",
			Paths:    []string{featurePath},
			TestingT: t, // Testing instance that will run subtests.
		},
	}

	if code := suite.Run(); code != 0 {
		t.Fatalf("status %d returned, failed to run %s", code, featurePath)
	}
}
