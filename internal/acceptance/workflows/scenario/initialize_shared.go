package scenario

import (
	"context"
	"regexp"

	"github.com/cucumber/godog"
)

// ================================================================
// Shared Intializers
// ----------------------------------------------------------------
//
// These are usable for a range of scenarios. They are organized by
// what they operate on: tiles, tile source code...

func InitializeExec(ctx *godog.ScenarioContext) { initializeExec(ctx) }
func initializeExec(ctx initializeContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return configureStandardFileDescriptors(ctx), nil
	})
	ctx.Step(regexp.MustCompile(`^(stdout|stderr|"[^"]*") contains substring: (.*)`), outputContainsSubstring)
	ctx.Step(regexp.MustCompile(`^I (try to )?invoke kiln version$`), iInvokeKilnVersion)
	ctx.Step(regexp.MustCompile(`^the exit code is (\d+)$`), theExitCodeIs)
}

// InitializeTile provides some basic tile and tile repo interaction steps.
//
// Most other steps require iHaveARepositoryCheckedOutAtRevision to have been run because it sets the tile repo path on the context.
func InitializeTile(ctx *godog.ScenarioContext) { initializeTile(ctx) }
func initializeTile(ctx initializeContext) {
	ctx.Step(regexp.MustCompile(`^a Tile is created$`), aTileIsCreated)
	ctx.Step(regexp.MustCompile(`^the Tile contains "([^"]*)"$`), theTileContains)
	ctx.Step(regexp.MustCompile(`^the Tile only contains compiled releases$`), theTileOnlyContainsCompiledReleases)
}

func InitializeTileSourceCode(ctx *godog.ScenarioContext) { initializeTileSourceCode(ctx) }
func initializeTileSourceCode(ctx initializeContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return setTileRepoPath(ctx, "hello-tile"), nil
	})
	ctx.After(resetTileRepository)

	ctx.Step(regexp.MustCompile(`^kiln validate succeeds$`), kilnValidateSucceeds)

	ctx.Step(regexp.MustCompile(`^I have a "([^"]*)" repository checked out at (.*)$`), iHaveARepositoryCheckedOutAtRevision)
	ctx.Step(regexp.MustCompile(`^the repository has no fetched releases$`), theRepositoryHasNoFetchedReleases)
	ctx.Step(regexp.MustCompile(`^I set the version constraint to "([^"]*)" for release "([^"]*)"$`), iSetAVersionConstraintForRelease)

	ctx.Step(regexp.MustCompile(`^the Kilnfile\.lock specifies version "([^"]*)" for release "([^"]*)"$`), theLockSpecifiesVersionForRelease)
}

func InitializeGitHub(ctx *godog.ScenarioContext) { initializeGitHub(ctx) }
func initializeGitHub(ctx initializeContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return loadGithubToken(ctx)
	})
	ctx.Step(regexp.MustCompile(`^GitHub repository "([^/]*)/([^"]*)" has release with tag "([^"]*)"$`), githubRepoHasReleaseWithTag)
}
