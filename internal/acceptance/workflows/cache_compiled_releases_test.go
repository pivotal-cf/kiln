package workflows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/cucumber/godog"
)

func TestCacheCompiledRelease(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: initializeCacheCompiledReleaseScenario,
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

func initializeCacheCompiledReleaseScenario(ctx *godog.ScenarioContext) {
	var scenario cacheCompiledReleaseScenario
	scenario.kilnBakeScenario.registerSteps(ctx)
	scenario.registerSteps(ctx)
}

type cacheCompiledReleaseScenario struct {
	kilnBakeScenario

	Environment struct {
		OpsManagerPrivateKey string `json:"ops_manager_private_key"`
		OpsManager           struct {
			Password string `json:"password"`
			URL      string `json:"url"`
			Username string `json:"username"`
		} `json:"ops_manager"`
		AvailabilityZones []string `json:"azs"`
		ServiceSubnetName string   `json:"service_subnet_name"`
	}
}

func (scenario *cacheCompiledReleaseScenario) registerSteps(ctx *godog.ScenarioContext) {
	ctx.Step(regexp.MustCompile(`^I add a compiled s3 release-source "([^"]*)" to the Kilnfile$`), scenario.iAddACompiledSReleasesourceToTheKilnfile)
	ctx.Step(regexp.MustCompile(`^I have a smith environment$`), scenario.iHaveASmithEnvironment)
	ctx.Step(regexp.MustCompile(`^I invoke kiln cache-compiled-releases$`), scenario.iInvokeKilnCachecompiledreleases)
	ctx.Step(regexp.MustCompile(`^I upload, configure, and apply the tile with stemcell ([^\/]*)\/(.*)$`), scenario.iUploadConfigureAndApplyTheTileWithStemcell)
}

func (scenario *cacheCompiledReleaseScenario) iAddACompiledSReleasesourceToTheKilnfile(bucketName string) error {
	return godog.ErrPending
}

func (scenario *cacheCompiledReleaseScenario) iHaveASmithEnvironment() error {
	if os.Getenv("TOOLSMITHS_ENVIRONMENT_NAME") == "" {
		return fmt.Errorf("TOOLSMITHS_ENVIRONMENT_NAME not set run `smith claim`")
	}

	smithRead := exec.Command("smith", "read")
	var smithReadOutput bytes.Buffer
	smithRead.Stdout = &smithReadOutput
	err := smithRead.Run()
	if err != nil {
		return err
	}
	err = json.Unmarshal(smithReadOutput.Bytes(), &scenario.Environment)
	if err != nil {
		return fmt.Errorf("failed to parse smith environment: %w", err)
	}

	return nil
}

func (scenario *cacheCompiledReleaseScenario) iInvokeKilnCachecompiledreleases() error {
	cmd := exec.Command("go", "run", "github.com/pivotal-cf/kiln", "cache-compiled-releases",
		"--variable", "github_token="+scenario.githubToken,
	)
	cmd.Dir = scenario.tilePath
	return runAndLogOnError(cmd)
}

func (scenario *cacheCompiledReleaseScenario) iUploadConfigureAndApplyTheTileWithStemcell(stemcellOS, stemcellVersion string) error {
	if len(scenario.Environment.AvailabilityZones) == 0 {
		return fmt.Errorf("[TODO: context about why 'we' expect this]expected availability zones")
	}

	err := runAndLogOnError(exec.Command("smith", "om", "--", "upload-product", "-p", scenario.defaultFilePathForTile()))
	if err != nil {
		return err
	}
	err = runAndLogOnError(exec.Command("smith", "om", "--", "stage-product", "--product-name", "hello", "--product-version", scenario.tileVersion))
	if err != nil {
		return err
	}
	err = runAndLogOnError(exec.Command("smith", "om", "--", "configure-product",
		"--config", "hello-product-config.yml",
		"--var", "subnet="+scenario.Environment.ServiceSubnetName,
		"--var", "az="+scenario.Environment.AvailabilityZones[0],
	))
	if err != nil {
		return err
	}
	err = runAndLogOnError(exec.Command("smith", "om", "--", "apply-changes"))
	if err != nil {
		return err
	}

	return nil
}

/*
- type: s3
    bucket: hello-tile-releases
    path_template: {{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
    region: us-west-1
    access_key_id: $(variable "aws_access_key_id")
    secret_access_key: $(variable "aws_secret_access_key")
*/
