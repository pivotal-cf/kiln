package workflows

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/pkg/proofing"
	"github.com/pivotal-cf/kiln/pkg/tile"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
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
		OpsManagerPrivateKey string
		OpsManager           struct {
			URL      string
			Password string
			Username string
		}
		AvailabilityZones []string
		ServiceSubnetName string
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
	ctx.Step(regexp.MustCompile(`^I invoke kiln cache-compiled-releases$`), scenario.iInvokeKilnCacheCompiledReleases)
	ctx.Step(regexp.MustCompile(`^I upload, configure, and apply the tile$`), scenario.iUploadConfigureAndApplyTheTile)
	ctx.Step(regexp.MustCompile(`^the stemcell version in the lock matches the used for the tile$`), scenario.theStemcellVersionInTheLockMatchesTheUsedForTheTile)
	ctx.Step(regexp.MustCompile(`^the Tile only contains compiled releases$`), scenario.theTileOnlyContainsCompiledReleases)
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

func (scenario *cacheCompiledReleaseScenario) iInvokeKilnCacheCompiledReleases() error {
	_, err := loadEnv("BOSH_ALL_PROXY")
	if err != nil {
		return err
	}

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
	var stemcellAssociations struct {
		StemcellLibrary []struct {
			Version string `yaml:"version"`
		} `yaml:"stemcell_library"`
	}
	err := runAndParseStdoutAsYAML(
		exec.Command("om", "--skip-ssl-validation",
			"curl", "--path", "/api/v0/stemcell_associations",
		),
		&stemcellAssociations,
	)
	if len(stemcellAssociations.StemcellLibrary) == 0 {
		return fmt.Errorf("no stemcells found on ops manager")
	}
	var kl cargo.KilnfileLock
	err = loadFileAsYAML(scenario.kilnfileLockPath(), &kl)
	if err != nil {
		return err
	}
	kl.Stemcell.Version = stemcellAssociations.StemcellLibrary[0].Version
	return saveAsYAML(scenario.kilnfileLockPath(), kl)
}

func (scenario *cacheCompiledReleaseScenario) iUploadConfigureAndApplyTheTile() error {
	if err := scenario.loadOpsManagerEnvironment(); err != nil {
		return err
	}

	err := runAndLogOnError(exec.Command("om", "--skip-ssl-validation", "upload-product", "--product", scenario.defaultFilePathForTile()))
	if err != nil {
		return err
	}
	err = runAndLogOnError(exec.Command("om", "--skip-ssl-validation", "stage-product", "--product-name", "hello", "--product-version", scenario.tileVersion))
	if err != nil {
		return err
	}
	err = runAndLogOnError(exec.Command("om", "--skip-ssl-validation", "configure-product",
		"--config", "hello-product-config.yml",
		"--var", "subnet="+scenario.Environment.ServiceSubnetName,
		"--var", "az="+scenario.Environment.AvailabilityZones[0],
	))
	if err != nil {
		return err
	}
	err = runAndLogOnError(exec.Command("om", "--skip-ssl-validation", "apply-changes"))
	if err != nil {
		return err
	}

	return nil
}

func (scenario *cacheCompiledReleaseScenario) loadOpsManagerEnvironment() error {
	var err error

	scenario.Environment.OpsManager.URL, err = loadEnv("OM_TARGET")
	if err != nil {
		return err
	}
	scenario.Environment.OpsManager.Username, err = loadEnv("OM_USERNAME")
	if err != nil {
		return err
	}
	scenario.Environment.OpsManager.Password, err = loadEnv("OM_PASSWORD")
	if err != nil {
		return err
	}
	scenario.Environment.OpsManagerPrivateKey, err = loadEnv("OM_PRIVATE_KEY")
	if err != nil {
		return err
	}

	var directorConfig struct {
		AvailabilityZoneConfiguration []struct {
			Name string `yaml:"name"`
		} `yaml:"az-configuration"`
		NetworkConfiguration struct {
			Networks []struct {
				Name string `yaml:"name"`
			} `yaml:"networks"`
		} `yaml:"networks-configuration"`
	}
	err = runAndParseStdoutAsYAML(
		exec.Command("om", "--skip-ssl-validation", "staged-director-config", "--no-redact"),
		&directorConfig,
	)

	for _, az := range directorConfig.AvailabilityZoneConfiguration {
		scenario.Environment.AvailabilityZones = append(scenario.Environment.AvailabilityZones, az.Name)
	}
	for _, network := range directorConfig.NetworkConfiguration.Networks {
		if !strings.HasSuffix(network.Name, "-services-subnet") {
			continue
		}
		scenario.Environment.ServiceSubnetName = network.Name
		break
	}

	return nil
}

func (scenario *cacheCompiledReleaseScenario) theTileOnlyContainsCompiledReleases() error {
	metadataBuf, err := tile.ReadMetadataFromFile(scenario.defaultFilePathForTile())
	if err != nil {
		return err
	}

	var metadata struct {
		Releases []proofing.Release `yaml:"releases"`
	}
	err = yaml.Unmarshal(metadataBuf, &metadata)
	if err != nil {
		return err
	}

	for _, release := range metadata.Releases {
		helloReleaseTarball := bytes.NewBuffer(nil)
		_, err := tile.ReadReleaseFromFile(scenario.defaultFilePathForTile(), release.Name, release.Version, helloReleaseTarball)
		if err != nil {
			return err
		}
		manifestBuf, err := readReleaseManifest(helloReleaseTarball)
		if err != nil {
			return err
		}
		err = validateAllPackagesAreCompiled(manifestBuf, release)
		if err != nil {
			return err
		}
	}

	return nil
}

// validateAllPackagesAreCompiled asserts that if any package is listed under "packages", it is not complied.
// It does not ensure "compiled_packages" is populated.
func validateAllPackagesAreCompiled(manifestBuf []byte, release proofing.Release) error {
	var manifest struct {
		Packages []struct {
			Name string `yaml:"name"`
		} `yaml:"packages"`
	}
	err := yaml.Unmarshal(manifestBuf, &manifest)
	if err != nil {
		return err
	}
	if len(manifest.Packages) > 0 {
		sb := new(strings.Builder)
		sb.WriteString(fmt.Sprintf("release %s/%s contains un-compiled packages: ", release.Name, release.Version))
		for i, p := range manifest.Packages {
			sb.WriteString(p.Name)
			if i < len(manifest.Packages)-1 {
				sb.WriteString(", ")
			}
		}
		return errors.New(sb.String())
	}
	return nil
}

func readReleaseManifest(r io.Reader) ([]byte, error) {
	const releaseManifestFileName = "release.MF"
	zipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(zipReader)

	for {
		h, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if path.Base(h.Name) != releaseManifestFileName {
			continue
		}
		return io.ReadAll(tarReader)
	}

	return nil, fmt.Errorf("%q not found", releaseManifestFileName)
}

func loadEnv(n string) (string, error) {
	v := os.Getenv(n)
	if v == "" {
		return "", fmt.Errorf("required env variable %s not set", n)
	}
	return v, nil
}

func closeAndIgnoreErr(c io.Closer) {
	_ = c.Close()
}

func runAndParseStdoutAsYAML(cmd *exec.Cmd, d interface{}) error {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		_, _ = io.Copy(os.Stdout, &stderr)
		return err
	}
	return yaml.Unmarshal(stdout.Bytes(), d)
}
