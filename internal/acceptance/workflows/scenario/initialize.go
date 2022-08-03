package scenario

import (
	"context"
	"regexp"

	"github.com/cucumber/godog"
)

// InitializeContext is based on *godog.ScenarioContext
type InitializeContext interface {
	Step(expr, stepFunc interface{})
	Before(h godog.BeforeScenarioHook)
	After(h godog.AfterScenarioHook)
}

var _ InitializeContext = (*godog.ScenarioContext)(nil)

func InitializeBake(ctx InitializeContext) {
	ctx.Step(regexp.MustCompile(`^I (try to )?invoke kiln bake$`), iInvokeKilnBake)
}

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
//   aws s3 ls s3://hello-tile-releases
//
// The OM_TARGET, OM_USERNAME, and OM_PASSWORD behave as they would with the OM CLI.
// You can ensure they are correct by running any om command. For example:
//
//   om staged-products
//
// Note, where the scenario uses the om CLI, the command inherits the parent's environment.
// So if needed you can set OM_SKIP_SSL_VALIDATION and other om environment variables.
//
// OM_PRIVATE_KEY is not a standard om environment variable; it is used by kiln not OM.
// To ensure it works you can execute:
//
//   echo "${OM_PRIVATE_KEY}" > /tmp/om.key
//   chmod 0400 /tmp/om.key
//   ssh -i /tmp/om.key "ubuntu@pcf.example.com"
//
func InitializeCacheCompiledReleases(ctx InitializeContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		_, _, err := loadS3Credentials()
		if err != nil {
			return ctx, err
		}
		return loadEnvironment(ctx)
	})
	ctx.Step(regexp.MustCompile(`^I add a compiled s3 release-source "([^"]*)" to the Kilnfile$`), iAddACompiledSReleaseSourceToTheKilnfile)
	ctx.Step(regexp.MustCompile(`^I (try to )?invoke kiln cache-compiled-releases$`), iInvokeKilnCacheCompiledReleases)
	ctx.Step(regexp.MustCompile(`^the stemcell version in the lock matches the used for the tile$`), theStemcellVersionInTheLockMatchesTheUsedForTheTile)
}

// InitializeEnvironment use the om CLI to upload, stage, and apply the test tile.
//
// The OM_TARGET, OM_USERNAME, and OM_PASSWORD behave as they would with the OM CLI.
// You can ensure they are correct by running any om command. For example:
//
//   om staged-products
//
func InitializeEnvironment(ctx InitializeContext) {
	ctx.Step(regexp.MustCompile(`^I upload, configure, and apply the tile$`), iUploadConfigureAndApplyTheTile)
}

// InitializeFetch requires a credentials to fetch releases.
//
// To check if the environment is configured properly run
//
//    echo "${GITHUB_TOKEN}"
//
// It should output a valid github token.
// If you have not set this, loadGithubToken will try to execute `gh auth status --show-token` and will parse and set the token from the output.
func InitializeFetch(ctx InitializeContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return loadGithubToken(ctx)
	})
	ctx.After(cleanUpFetchedReleases)
	ctx.Step(regexp.MustCompile(`^I (try to )?invoke kiln fetch$`), iInvokeKilnFetch)
}

// InitializeTile provides some basic tile and tile repo interacation steps.
//
// Most other steps require iHaveARepositoryCheckedOutAtRevision to have been run because it sets the tile repo path on the context.
func InitializeTile(ctx InitializeContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return setTileRepoPath(ctx, "hello-tile"), nil
	})
	ctx.After(checkoutMainOnTileRepo)
	ctx.Step(regexp.MustCompile(`^a Tile is created$`), aTileIsCreated)
	ctx.Step(regexp.MustCompile(`^I have a "([^"]*)" repository checked out at (.*)$`), iHaveARepositoryCheckedOutAtRevision)
	ctx.Step(regexp.MustCompile(`^the repository has no fetched releases$`), theRepositoryHasNoFetchedReleases)
	ctx.Step(regexp.MustCompile(`^the Tile contains "([^"]*)"$`), theTileContains)
	ctx.Step(regexp.MustCompile(`^the Tile only contains compiled releases$`), theTileOnlyContainsCompiledReleases)
	ctx.Step(regexp.MustCompile(`^I set the version constraint to "([^"]*)" for release "([^"]*)"$`), iSetAVersionConstraintForRelease)

	ctx.Step(regexp.MustCompile(`^the Kilnfile\.lock specifies version "([^"]*)" for release "([^"]*)"$`), theLockSpecifiesVersionForRelease)
}

func InitializeValidate(ctx InitializeContext) {
	ctx.Step(regexp.MustCompile(`^kiln validate succeeds$`), kilnValidateSucceeds)
}

func InitializeFindReleaseVersion(ctx InitializeContext) {
	ctx.Step(`^I (try to )?invoke kiln find-release-version for "([^"]*)"$`, iInvokeKilnFindReleaseVersion)
}

func InitializeUpdateRelease(ctx InitializeContext) {
	ctx.Step(`^I (try to )?invoke kiln update-release for releas "([^"]*)" with version "([^"]*)"$`, iInvokeKilnUpdateRelease)
}

func InitializeGitHub(ctx InitializeContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return loadGithubToken(ctx)
	})
	ctx.Step(regexp.MustCompile(`^GitHub repository "([^/]*)/([^"]*)" has release with tag "([^"]*)"$`), githubRepoHasReleaseWithTag)
}

func InitializeHelp(ctx InitializeContext) {
	ctx.Step(regexp.MustCompile(`^I (try to )?invoke kiln help$`), iInvokeKilnHelp)
	ctx.Step(regexp.MustCompile(`^I (try to )?invoke kiln boo-boo$`), iInvokeKilnBooBoo)
	ctx.Step(regexp.MustCompile(`^I (try to )?invoke kiln (\S*) --boo-boo$`), iInvokeKilnCommandWithFlagBooBoo)
}

func InitializeExec(ctx InitializeContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return configureStandardFileDescriptors(ctx), nil
	})
	ctx.Step(regexp.MustCompile("^(stdout|stderr) contains substring: (.*)"), outputContainsSubstring)
	ctx.Step(regexp.MustCompile(`^I (try to )?invoke kiln version$`), iInvokeKilnVersion)
	ctx.Step(regexp.MustCompile(`^the exit code is (\d+)$`), theExitCodeIs)
}
