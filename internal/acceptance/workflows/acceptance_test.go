//go:build acceptance

// Package workflows executes cucumber style acceptance tests.
//
// To run the tests execute:
//
//    go test -v --tags acceptance github.com/pivotal-cf/kiln/internal/acceptance/workflows
//
// To run a particular test execute (notice the run tag value is a case-sensitive regular expression):
//
//    go test --run baking -v --tags acceptance github.com/pivotal-cf/kiln/internal/acceptance/workflows
//
package workflows

import (
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"github.com/pivotal-cf/kiln/internal/acceptance/workflows/scenario"
)

func Test_baking(t *testing.T) {
	testFeature(t,
		scenario.InitializeFetch,
		scenario.InitializeBake,
	)
}
func Test_command(t *testing.T) {
	testFeature(t,
		scenario.InitializeHelp,
	)
}
func Test_updating_releases(t *testing.T) {
	testFeature(t,
		scenario.InitializeGitHub,
		scenario.InitializeFindReleaseVersion,
		scenario.InitializeUpdateRelease,
	)
}
func Test_compiled_releases(t *testing.T) {
	testFeature(t,
		scenario.InitializeFetch,
		scenario.InitializeBake,
		scenario.InitializeCacheCompiledReleases,
	)
}

func Test_updating_stemcell(t *testing.T) {
	t.SkipNow()
	testFeature(t,
		scenario.InitializeFetch,
		scenario.InitializeBake,
	)
}

func testFeature(t *testing.T, initializers ...func(ctx scenario.InitializeContext)) {
	trimmedTestFuncName := strings.TrimPrefix(t.Name(), "Test_")
	featurePath := trimmedTestFuncName + ".feature"

	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			// default initializers
			scenario.InitializeExec(ctx)
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
