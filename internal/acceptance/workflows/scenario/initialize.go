package scenario

import (
	"context"
	"regexp"

	"github.com/cucumber/godog"
)

// scenarioContext is based on *godog.ScenarioContext
type scenarioContext interface {
	Step(expr, stepFunc any)
	Before(h godog.BeforeScenarioHook)
	After(h godog.AfterScenarioHook)
}

// scenarioContext exposes the subset of methods on *godog.ScenarioContext that we use.
// It is here because we want to have a bit of testing for the initialize functions.
var _ scenarioContext = (*godog.ScenarioContext)(nil)

// InitializeAWS used default AWS environment variables so AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be set.
// The credentials provided in those environment variables should be able to put, list, and delete objects on the bucket.
func InitializeAWS(ctx *godog.ScenarioContext) { initializeAWS(ctx) }

func initializeAWS(ctx scenarioContext) {
	ctx.Step(regexp.MustCompile(`^I remove all the objects in the bucket "([^"]+)"$`), iRemoveAllTheObjectsInBucket)
}

func InitializeEnv(ctx *godog.ScenarioContext) { initializeEnv(ctx) }
func initializeEnv(ctx scenarioContext) {
	ctx.Step(regexp.MustCompile(`^the environment variable "([^"]+)" is set$`), theEnvironmentVariableIsSet)
}

func InitializeExec(ctx *godog.ScenarioContext) { initializeExec(ctx) }
func initializeExec(ctx scenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return configureStandardFileDescriptors(ctx), nil
	})
	ctx.Step(regexp.MustCompile("^I execute (.*)$"), iExecute)
	ctx.Step(regexp.MustCompile(`^(stdout|stderr|"[^"]*") contains substring: (.*)`), outputContainsSubstring)
	ctx.Step(regexp.MustCompile(`^the exit code is (\d+)$`), theExitCodeIs)
	ctx.Step(regexp.MustCompile(`^I write file ("[^"]*")$`), iWriteFileWith)
}

func InitializeGitHub(ctx *godog.ScenarioContext) { initializeGitHub(ctx) }
func initializeGitHub(ctx scenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return loadGithubToken(ctx)
	})
	ctx.Step(regexp.MustCompile(`^GitHub repository "([^/]*)/([^"]*)" has release with tag "([^"]*)"$`), githubRepoHasReleaseWithTag)
}

func InitializeKiln(ctx *godog.ScenarioContext) { initializeKiln(ctx) }
func initializeKiln(ctx scenarioContext) {
	ctx.Step(regexp.MustCompile(`^I invoke kiln$`), iInvokeKiln)
	ctx.Step(regexp.MustCompile(`^I try to invoke kiln$`), iTryToInvokeKiln)
}

func InitializeRegex(ctx *godog.ScenarioContext) { initializeRegex(ctx) }
func initializeRegex(ctx scenarioContext) {
	ctx.Step(regexp.MustCompile(`^(stdout|stderr|"[^"]*") has regexp? matches: (.*)$`), hasRegexMatches)
}

func InitializeTanzuNetwork(ctx *godog.ScenarioContext) { initializeTanzuNetwork(ctx) }
func initializeTanzuNetwork(ctx scenarioContext) {
	ctx.Step(regexp.MustCompile(`^TanzuNetwork has product "([^"]*)" with version "([^"]*)"$`), tanzuNetworkHasProductWithVersion)
}

// InitializeTile provides some basic tile and tile repo interaction steps.
//
// Most other steps require iHaveARepositoryCheckedOutAtRevision to have been run because it sets the tile repo path on the context.
func InitializeTile(ctx *godog.ScenarioContext) { initializeTile(ctx) }

func initializeTile(ctx scenarioContext) {
	ctx.Step(regexp.MustCompile(`^a Tile is created$`), aTileIsCreated)
	ctx.Step(regexp.MustCompile(`^the Tile contains$`), theTileContains)
	ctx.Step(regexp.MustCompile(`^the Tile only contains compiled releases$`), theTileOnlyContainsCompiledReleases)
}

func InitializeTileSourceCode(ctx *godog.ScenarioContext) { initializeTileSourceCode(ctx) }
func initializeTileSourceCode(ctx scenarioContext) {
	ctx.After(resetTileRepository)

	ctx.Step(regexp.MustCompile(`^kiln validate succeeds$`), kilnValidateSucceeds)

	ctx.Step(regexp.MustCompile(`^I have a tile source directory "([^"]*)"$`), iHaveATileDirectory)

	ctx.Step(regexp.MustCompile(`^the repository has no fetched releases$`), theRepositoryHasNoFetchedReleases)

	ctx.Step(regexp.MustCompile(`^I set the version constraint to "([^"]*)" for release "([^"]*)"$`), iSetAVersionConstraintForRelease)
	ctx.Step(regexp.MustCompile(`^I set the Kilnfile stemcell version constraint to "([^"]*)"$`), iSetTheKilnfileStemcellVersionConstraint)

	ctx.Step(regexp.MustCompile(`^the Kilnfile\.lock specifies version "([^"]*)" for release "([^"]*)"$`), theLockSpecifiesVersionForRelease)
	ctx.Step(regexp.MustCompile(`^the Kilnfile\.lock specifies version "([^"]*)" for the stemcell$`), theLockStemcellVersionIs)

	ctx.Step(regexp.MustCompile(`^the Kilnfile version for release "([^"]*)" is "([^"]*)"$`), theKilnfileVersionForReleaseIs)
	ctx.Step(regexp.MustCompile(`^the Kilnfile version for the stemcell is "([^"]*)"$`), theKilnfileVersionForTheStemcellIs)
}
