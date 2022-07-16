package workflows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
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

	targetReleaseSource string

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

func (scenario *cacheCompiledReleaseScenario) kilnfilePath() string {
	return filepath.Join("hello-tile", "Kilnfile")
}

func (scenario *cacheCompiledReleaseScenario) kilnfileLockPath() string {
	return scenario.kilnfilePath() + ".lock"
}

func (scenario *cacheCompiledReleaseScenario) registerSteps(ctx *godog.ScenarioContext) {
	ctx.Step(regexp.MustCompile(`^I add a compiled s3 release-source "([^"]*)" to the Kilnfile$`), scenario.iAddACompiledSReleaseSourceToTheKilnfile)
	ctx.Step(regexp.MustCompile(`^I have a smith environment$`), scenario.iHaveASmithEnvironment)
	ctx.Step(regexp.MustCompile(`^I invoke kiln cache-compiled-releases$`), scenario.iInvokeKilnCacheCompiledReleases)
	ctx.Step(regexp.MustCompile(`^I upload, configure, and apply the tile with stemcell ([^\/]*)\/(.*)$`), scenario.iUploadConfigureAndApplyTheTileWithStemcell)
	ctx.Step(regexp.MustCompile(`^the stemcell version in the lock matches the used for the tile$`), scenario.theStemcellVersionInTheLockMatchesTheUsedForTheTile)
	ctx.Step(regexp.MustCompile(`^the Tile only contains compiled releases$`), theTileOnlyContainsCompiledReleases)

}

func (scenario *cacheCompiledReleaseScenario) iAddACompiledSReleaseSourceToTheKilnfile(bucketName string) error {
	_, err := checkEnvVar("AWS_ACCESS_KEY_ID", "required for s3 release source to cache releases")
	if err != nil {
		return err
	}
	_, err = checkEnvVar("AWS_SECRET_ACCESS_KEY", "required for s3 release source to cache releases")
	if err != nil {
		return err
	}

	var kf cargo.Kilnfile
	err = loadFileAsYAML(scenario.kilnfilePath(), &kf)
	if err != nil {
		return err
	}

	for _, rs := range kf.ReleaseSources {
		if rs.Bucket == bucketName {
			return nil
		}
	}

	scenario.targetReleaseSource = bucketName

	kf.ReleaseSources = append(kf.ReleaseSources, cargo.ReleaseSourceConfig{
		Type:            component.ReleaseSourceTypeS3,
		Bucket:          bucketName,
		PathTemplate:    "{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz",
		Region:          "us-west-1",
		AccessKeyId:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	})

	return saveAsYAML(scenario.kilnfilePath(), kf)
}

func loadFileAsYAML(filePath string, v any) error {
	kfBuf, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(kfBuf, v)
	if err != nil {
		return fmt.Errorf("failed to parse kilnfile: %w", err)
	}
	return nil
}

func saveAsYAML(filePath string, v any) error {
	kfBuf, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to render kilnfile: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer closeAndIgnoreErr(f)

	_, err = f.Write(kfBuf)
	return err
}

func checkEnvVar(name, message string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("%s is not set (%s)", name, message)
	}
	return value, nil
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

func (scenario *cacheCompiledReleaseScenario) iInvokeKilnCacheCompiledReleases() error {
	cmd := exec.Command("go", "run", "github.com/pivotal-cf/kiln", "cache-compiled-releases",
		"--variable", "github_token="+scenario.githubToken,
		"--upload-target-id", scenario.targetReleaseSource,
		"--name", "hello",
		"--om-username", scenario.Environment.OpsManager.Username,
		"--om-password", scenario.Environment.OpsManager.Password,
		"--om-target", scenario.Environment.OpsManager.URL,
		"--om-private-key", scenario.Environment.OpsManagerPrivateKey,
	)
	cmd.Dir = scenario.tilePath
	return runAndLogOnError(cmd)
}

func (scenario *cacheCompiledReleaseScenario) theStemcellVersionInTheLockMatchesTheUsedForTheTile() error {
	cmd := exec.Command("smith", "om", "--",
		"curl", "--path", "/api/v0/stemcell_associations",
	)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	var result struct {
		StemcellLibrary []struct {
			Version string `json:"version"`
		} `json:"stemcell_library"`
	}
	err := cmd.Run()
	if err != nil {
		return err
	}
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		return err
	}
	if len(result.StemcellLibrary) == 0 {
		return fmt.Errorf("no stemcells found on ops manager")
	}
	var kl cargo.KilnfileLock
	err = loadFileAsYAML(scenario.kilnfileLockPath(), &kl)
	if err != nil {
		return err
	}
	kl.Stemcell.Version = result.StemcellLibrary[0].Version
	return saveAsYAML(scenario.kilnfileLockPath(), kl)
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

func theTileOnlyContainsCompiledReleases() error {
	return godog.ErrPending
}

/*
- type: s3
    bucket: hello-tile-releases
    path_template: {{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
    region: us-west-1
    access_key_id: $(variable "aws_access_key_id")
    secret_access_key: $(variable "aws_secret_access_key")
*/

func closeAndIgnoreErr(c io.Closer) {
	_ = c.Close()
}
