//go:build acceptance

// Package workflows executes cucumber style acceptance tests.
//
// To run the tests execute:
//
//    go test -v --tags acceptance github.com/pivotal-cf/kiln/internal/acceptance/workflows
//
// To run a particular test execute (notice the run tag value is a case-sensitive regular expression):
//
//    go test --run bake -v --tags acceptance github.com/pivotal-cf/kiln/internal/acceptance/workflows
//
package workflows

import (
	"testing"

	"github.com/cucumber/godog"

	"github.com/pivotal-cf/kiln/internal/acceptance/workflows/steps"
)

func TestBake(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			steps.InitializeTile(ctx)
			steps.InitializeFetch(ctx)
			steps.InitializeBake(ctx)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"bake_test.feature"},
			TestingT: t, // Testing instance that will run subtests.
		},
	}

	if code := suite.Run(); code != 0 {
		t.Fatalf("status %d returned, failed to run feature tests", code)
	}
}

func TestCacheCompiledReleases(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			steps.InitializeTile(ctx)
			steps.InitializeFetch(ctx)
			steps.InitializeBake(ctx)
			steps.InitializeEnvironment(ctx)
			steps.InitializeCacheCompiledReleases(ctx)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"cache_compiled_releases_test.feature"},
			TestingT: t, // Testing instance that will run subtests.
		},
	}

	if code := suite.Run(); code != 0 {
		t.Fatalf("status %d returned, failed to run feature tests", code)
	}
}
