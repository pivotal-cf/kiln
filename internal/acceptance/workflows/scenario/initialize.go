package scenario

import (
	"context"
	"regexp"

	"github.com/cucumber/godog"
)

// scenarioContext is based on *godog.ScenarioContext
type scenarioContext interface {
	Step(expr, stepFunc interface{})
	Before(h godog.BeforeScenarioHook)
	After(h godog.AfterScenarioHook)
}

// scenarioContext exposes the subset of methods on *godog.ScenarioContext that we use.
// It is here because we want to have a bit of testing for the initialize functions.
var _ scenarioContext = (*godog.ScenarioContext)(nil)

// InitializeCacheCompiledReleases requires environment configuration to interact with a Tanzu Ops Manager.
//
// # Environment
//
//   - AWS_ACCESS_KEY_ID: credentials with access to an empty S3 bucket where the release will be cached
//   - AWS_SECRET_ACCESS_KEY: credentials with access to empty an S3 bucket where the release will be cached
//   - BOSH_ALL_PROXY: this environment variable is required by the BOSH client used in Kiln. To see how to construct it see [BOSH CLI Tunneling]: https://bosh.io/docs/cli-tunnel/
//   - OM_TARGET: should be set to a url like https://pcf.example.com
//   - OM_USERNAME: should be set to the Ops Manager username
//   - OM_PASSWORD: should be set to the Ops Manager password
//   - OM_PRIVATE_KEY: should be set with a private key in PEM format that can be used to ssh to the ops manager
//
// ## Debugging
//
// The AWS credentials are the default environment variables for the AWS CLI.
// Note you can change the bucket used for testing by changing the value in the feature file.
// So you can check if they will work by invoking the following command:
//
//	aws s3 ls s3://hello-tile-releases
//
// The OM_TARGET, OM_USERNAME, and OM_PASSWORD behave as they would with the OM CLI.
// You can ensure they are correct by running any om command. For example:
//
//	om staged-products
//
// Note, where the scenario uses the om CLI, the command inherits the parent's environment.
// So if needed you can set OM_SKIP_SSL_VALIDATION and other om environment variables.
//
// OM_PRIVATE_KEY is not a standard om environment variable; it is used by kiln not OM.
// To ensure it works you can execute:
//
//	echo "${OM_PRIVATE_KEY}" > /tmp/om.key
//	chmod 0400 /tmp/om.key
//	ssh -i /tmp/om.key "ubuntu@pcf.example.com"
func InitializeCacheCompiledReleases(ctx *godog.ScenarioContext) {
	initializeCacheCompiledReleases(ctx)
}
func initializeCacheCompiledReleases(ctx scenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		_, _, err := loadS3Credentials()
		if err != nil {
			return ctx, err
		}
		return loadEnvironment(ctx)
	})
	ctx.Step(regexp.MustCompile(`^I add a compiled s3 release-source "([^"]*)" to the Kilnfile$`), iAddACompiledSReleaseSourceToTheKilnfile)
	ctx.Step(regexp.MustCompile(`^I set the stemcell version in the lock to match the one used for the tile$`), iSetTheStemcellVersionInTheLockToMatchTheOneUsedForTheTile)
	ctx.Step(regexp.MustCompile(`^I upload, configure, and apply the tile$`), iUploadConfigureAndApplyTheTile)
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
	ctx.Step(regexp.MustCompile(`^(stdout|stderr|"[^"]*") contains substring: (.*)`), outputContainsSubstring)
	ctx.Step(regexp.MustCompile(`^the exit code is (\d+)$`), theExitCodeIs)
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
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return setTileRepoPath(ctx, "hello-tile"), nil
	})
	ctx.After(resetTileRepository)

	ctx.Step(regexp.MustCompile(`^kiln validate succeeds$`), kilnValidateSucceeds)

	ctx.Step(regexp.MustCompile(`^I have a "([^"]*)" repository checked out at (.*)$`), iHaveARepositoryCheckedOutAtRevision)

	ctx.Step(regexp.MustCompile(`^the repository has no fetched releases$`), theRepositoryHasNoFetchedReleases)

	ctx.Step(regexp.MustCompile(`^I set the version constraint to "([^"]*)" for release "([^"]*)"$`), iSetAVersionConstraintForRelease)
	ctx.Step(regexp.MustCompile(`^I set the Kilnfile stemcell version constraint to "([^"]*)"$`), iSetTheKilnfileStemcellVersionConstraint)

	ctx.Step(regexp.MustCompile(`^the Kilnfile\.lock specifies version "([^"]*)" for release "([^"]*)"$`), theLockSpecifiesVersionForRelease)
	ctx.Step(regexp.MustCompile(`^the Kilnfile\.lock specifies version "([^"]*)" for the stemcell$`), theLockStemcellVersionIs)
}

func InitializeRegex(ctx *godog.ScenarioContext) { initializeRegex(ctx) }
func initializeRegex(ctx scenarioContext) {
	ctx.Step(regexp.MustCompile(`^(stdout|stderr|"[^"]*") has regexp? matches: (.*)$`), hasRegexMatches)
}
