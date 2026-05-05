package test

import (
	"archive/tar"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

const (
	// MinimumDockerServerVersion the test was failing with an older version this may be a bit conservative.
	// If the integration tests pass on your machine with an older version, feel free to PR a less conservative value.
	MinimumDockerServerVersion = "> 24.0.0"
	MinimumPodmanServerVersion = "> 5.3.0"

	// DockerVirtualRegistryHost is the docker-virtual registry used in Dockerfile FROM lines
	// and in ImageBuild AuthConfigs. Keep in sync with internal/test/Dockerfile.
	DockerVirtualRegistryHost = "tas-rel-eng-docker-virtual.usw1.packages.broadcom.com"
)

func Run(ctx context.Context, w io.Writer, configuration Configuration) error {
	dockerDaemon, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	return runTest(ctx, w, dockerDaemon, configuration)
}

type Configuration struct {
	SSHSocketAddress string

	AbsoluteTileDirectory string

	RunAll,
	RunMigrations,
	RunManifest,
	RunStability bool

	GinkgoFlags string
	Environment []string
	Verbose     bool
}

// testPlan holds the complete set of shell work for a kiln test run: global
// setup that must succeed before any suite, and an ordered list of named
// suites each of which is run as an independent unit.
type testPlan struct {
	setup  []string // fail-fast preamble (git config, etc.)
	suites []suiteStep
}

// suiteStep is one test suite — migrations, stability, or manifest.
// cmds are chained with && inside a subshell so their exit code is captured
// as a single unit.
type suiteStep struct {
	name string   // human label used in header and summary
	cmds []string // shell commands; first entry is typically the header printf
}

// suiteHeader builds the "Running Suite: …\n…\n" printf command for a suite.
func suiteHeader(title string) string {
	label := "Running Suite: " + title
	return fmt.Sprintf(`printf '\n%s\n%s\n'`, label, strings.Repeat("=", len(label)))
}

func (configuration Configuration) commands() (testPlan, error) {
	if !filepath.IsAbs(configuration.AbsoluteTileDirectory) {
		return testPlan{}, fmt.Errorf("tile path must be absolute")
	}
	tileDirName := filepath.Base(configuration.AbsoluteTileDirectory)

	plan := testPlan{
		setup: []string{"git config --global --add safe.directory '*'"},
	}

	if configuration.RunMigrations || configuration.RunAll {
		plan.suites = append(plan.suites, suiteStep{
			name: "Migration Tests",
			cmds: []string{
				fmt.Sprintf("cd /tas/%s/migrations", tileDirName),
				npmInstallCommand(configuration.AbsoluteTileDirectory, configuration.Verbose),
				suiteHeader("Migration Tests"),
				"npm test",
			},
		})
	}

	// Each ginkgo suite gets its own invocation so output is never interleaved.
	// Stability tests use Go's standard testing package (no ginkgo bootstrap), so
	// ginkgo does not print "Running Suite: ..."; we add the header ourselves.
	// Manifest suites use ginkgo specs and print their own header — we only add a
	// blank line. Note: not compatible with tiles that use ginkgo v2.
	if configuration.RunStability || configuration.RunAll {
		stabilityPath := fmt.Sprintf("/tas/%s/test/stability", tileDirName)
		plan.suites = append(plan.suites, suiteStep{
			name: "Stability Tests",
			cmds: []string{
				suiteHeader("Stability Tests"),
				fmt.Sprintf("cd /tas/%s && ginkgo %s %s", tileDirName, configuration.GinkgoFlags, stabilityPath),
			},
		})
	}

	if configuration.RunManifest || configuration.RunAll {
		manifestPath := fmt.Sprintf("/tas/%s/test/manifest", tileDirName)
		plan.suites = append(plan.suites, suiteStep{
			name: "Manifest Tests",
			cmds: []string{
				`printf '\n'`,
				fmt.Sprintf("cd /tas/%s && ginkgo %s %s", tileDirName, configuration.GinkgoFlags, manifestPath),
			},
		})
	}

	return plan, nil
}

// script produces the complete bash command for the test container.
//
// Each suite runs in a subshell so cd calls don't leak between suites. Exit
// codes are captured individually. When more than one suite is selected a
// colored pass/fail summary is printed at the end. The script always exits
// non-zero if any suite failed. When verbose is true, start/end timestamps
// are echoed before and after each suite.
func (p testPlan) script(verbose bool) string {
	var b strings.Builder

	// Global setup — fail fast on any error.
	if len(p.setup) > 0 {
		b.WriteString(strings.Join(p.setup, " && "))
		b.WriteString("\n")
	}

	if len(p.suites) == 0 {
		return b.String()
	}

	// One subshell per suite; exit code in _exitN. End-time (_timeN) only
	// captured when verbose — it is only used in the verbose summary format.
	for i, s := range p.suites {
		if verbose {
			fmt.Fprintf(&b, "\necho \"[$(date '+%%H:%%M:%%S')] Starting: %s\"\n", s.name)
		}
		fmt.Fprintf(&b, "\n(%s); _exit%d=$?\n", strings.Join(s.cmds, " && "), i)
		if verbose {
			fmt.Fprintf(&b, "_time%d=$(date '+%%H:%%M:%%S')\n", i)
			fmt.Fprintf(&b, "echo \"[$_time%d] Completed: %s\"\n", i, s.name)
		}
	}

	// Summary — only when running more than one suite.
	if len(p.suites) > 1 {
		b.WriteString("\nprintf '\\n'\n")
		for i, s := range p.suites {
			if verbose {
				fmt.Fprintf(&b,
					"[ $_exit%d -eq 0 ] && printf '[%%s] \\033[32m✓\\033[0m %s Passed\\n' \"$_time%d\" || printf '[%%s] \\033[31m✗\\033[0m %s Failed\\n' \"$_time%d\"\n",
					i, s.name, i, s.name, i,
				)
			} else {
				fmt.Fprintf(&b,
					"[ $_exit%d -eq 0 ] && printf '\\033[32m✓\\033[0m %s Passed\\n' || printf '\\033[31m✗\\033[0m %s Failed\\n'\n",
					i, s.name, s.name,
				)
			}
		}
	}

	// Overall exit — non-zero if any suite failed.
	b.WriteString("\n_overall=0\n")
	for i := range p.suites {
		fmt.Fprintf(&b, "[ $_exit%d -ne 0 ] && _overall=1\n", i)
	}
	b.WriteString("exit $_overall\n")

	return b.String()
}

//counterfeiter:generate -o ./fakes/moby_client.go --fake-name MobyClient . mobyClient
type mobyClient interface {
	ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (build.ImageBuildResponse, error)
	Ping(ctx context.Context) (types.Ping, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specV1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
}

func runTest(ctx context.Context, w io.Writer, dockerDaemon mobyClient, configuration Configuration) error {
	_, err := dockerDaemon.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	plan, err := configuration.commands()
	if err != nil {
		return err
	}

	envMap, err := decodeEnvironment(configuration.Environment)
	if err != nil {
		return fmt.Errorf("failed to parse environment: %w", err)
	}

	username, password, err := requiredArtifactoryCredentialsFromMap(envMap)
	if err != nil {
		return err
	}
	envMap["ARTIFACTORY_USERNAME"] = username
	envMap["ARTIFACTORY_PASSWORD"] = password

	if err := buildTestImage(ctx, w, dockerDaemon, username, password, envMap); err != nil {
		return err
	}

	parentDir := filepath.Dir(configuration.AbsoluteTileDirectory)
	tileDir := filepath.Base(configuration.AbsoluteTileDirectory)
	envVars := getTileTestEnvVars(configuration.AbsoluteTileDirectory, tileDir, envMap)

	return startAndWaitContainer(ctx, w, dockerDaemon, plan.script(configuration.Verbose), envVars, parentDir, configuration.Verbose)
}

// buildTestImage builds the kiln test Docker image, forwarding Artifactory
// credentials as build args and registry auth for pulling base images.
func buildTestImage(ctx context.Context, w io.Writer, dockerDaemon mobyClient, username, password string, envMap environmentVars) error {
	var dockerfileTarball bytes.Buffer
	if err := createDockerfileTarball(tar.NewWriter(&dockerfileTarball), dockerfile); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(w, "Preparing test image...")
	resp, err := dockerDaemon.ImageBuild(ctx, &dockerfileTarball, build.ImageBuildOptions{
		Tags: []string{"kiln_test_dependencies:vmware"},
		BuildArgs: map[string]*string{
			"ARTIFACTORY_USERNAME": &username,
			"ARTIFACTORY_PASSWORD": &password,
		},
		AuthConfigs:    registryAuthForDockerVirtual(envMap),
		SuppressOutput: true,
	})
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	if err := checkImageBuildResponse(resp.Body, nil); err != nil {
		return fmt.Errorf("image build failed: %w", err)
	}
	return nil
}

// startAndWaitContainer creates, starts, and waits for the test container to
// exit, streaming its logs to w. It stops the container on SIGINT.
func startAndWaitContainer(ctx context.Context, w io.Writer, dockerDaemon mobyClient, script string, envVars environmentVars, parentDir string, verbose bool) error {
	testContainer, err := dockerDaemon.ContainerCreate(ctx, &container.Config{
		Image: "kiln_test_dependencies:vmware",
		Cmd:   []string{"/bin/bash", "-c", script},
		Env:   encodeEnvironment(envVars),
		Tty:   true,
	}, &container.HostConfig{
		LogConfig: container.LogConfig{
			Config: map[string]string{
				"mode": string(container.LogModeNonBlock),
			},
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: parentDir,
				Target: "/tas",
			},
		},
		AutoRemove: true,
	}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	if verbose {
		_, _ = fmt.Fprintf(w, "Container: %s\n", testContainer.ID)
	}

	errG := errgroup.Group{}

	sigInt := make(chan os.Signal, 1)
	signal.Notify(sigInt, os.Interrupt)
	errG.Go(func() error {
		<-sigInt
		err := dockerDaemon.ContainerStop(ctx, testContainer.ID, container.StopOptions{
			Signal: "SIGKILL",
		})
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				return nil
			}
			if strings.Contains(err.Error(), "no such container") {
				return nil
			}
			return fmt.Errorf("failed to stop container: %w", err)
		}
		return nil
	})

	if err := dockerDaemon.ContainerStart(ctx, testContainer.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start test container: %w", err)
	}

	// Subscribe for exit before draining logs. With AutoRemove, the engine may delete the
	// container as soon as it stops; waiting for removal after io.Copy can race and
	// return "no such container" (often under Podman). next-exit records the exit while
	// the ID still exists.
	statusCh, containerWaitError := dockerDaemon.ContainerWait(ctx, testContainer.ID, container.WaitConditionNextExit)

	out, err := dockerDaemon.ContainerLogs(ctx, testContainer.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return fmt.Errorf("container log request failure: %w", err)
	}
	_, _ = fmt.Fprintln(w, "")
	if _, err := io.Copy(w, out); err != nil {
		return err
	}

	var resultErr error
	select {
	case err := <-containerWaitError:
		resultErr = err
	case status := <-statusCh:
		if status.StatusCode != 0 {
			if status.Error != nil {
				resultErr = fmt.Errorf("test failed with exit code %d: %s", status.StatusCode, status.Error.Message)
			} else {
				resultErr = fmt.Errorf("test failed with exit code %d", status.StatusCode)
			}
		}
	}
	signal.Stop(sigInt)
	close(sigInt)

	return errors.Join(resultErr, errG.Wait())
}

type environmentVars = map[string]string

func encodeEnvironment(m environmentVars) []string {
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, strings.Join([]string{k, v}, "="))
	}
	return result
}

// RequiredArtifactoryCredentials resolves ARTIFACTORY_USERNAME and ARTIFACTORY_PASSWORD
// from -e flags and os.Getenv for kiln test.
func RequiredArtifactoryCredentials(envVarArgs []string) (username, password string, err error) {
	m, err := decodeEnvironment(envVarArgs)
	if err != nil {
		return "", "", err
	}
	return requiredArtifactoryCredentialsFromMap(m)
}

// requiredArtifactoryCredentialsFromMap resolves credentials from an already-decoded
// environment map, falling back to os.Getenv when a value is absent.
func requiredArtifactoryCredentialsFromMap(m environmentVars) (username, password string, err error) {
	user := strings.TrimSpace(m["ARTIFACTORY_USERNAME"])
	if user == "" {
		user = strings.TrimSpace(os.Getenv("ARTIFACTORY_USERNAME"))
	}
	pass := strings.TrimSpace(m["ARTIFACTORY_PASSWORD"])
	if pass == "" {
		pass = strings.TrimSpace(os.Getenv("ARTIFACTORY_PASSWORD"))
	}
	if user == "" {
		return "", "", fmt.Errorf("kiln test requires ARTIFACTORY_USERNAME: set it using -e or export it in your environment")
	}
	if pass == "" {
		return "", "", fmt.Errorf("kiln test requires ARTIFACTORY_PASSWORD: set it using -e or export it in your environment")
	}
	return user, pass, nil
}

// registryAuthForDockerVirtual supplies credentials for pulling FROM images on
// DockerVirtualRegistryHost during docker build (X-Registry-Config).
func registryAuthForDockerVirtual(env environmentVars) map[string]registry.AuthConfig {
	user := strings.TrimSpace(env["ARTIFACTORY_USERNAME"])
	pass := strings.TrimSpace(env["ARTIFACTORY_PASSWORD"])
	if user == "" || pass == "" {
		return nil
	}
	host := DockerVirtualRegistryHost
	return map[string]registry.AuthConfig{
		host: {
			Username:      user,
			Password:      pass,
			ServerAddress: host,
		},
	}
}

func decodeEnvironment(environmentVarArgs []string) (environmentVars, error) {
	envMap := make(environmentVars)
	for _, envVar := range environmentVarArgs {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("environment variables must have the format [key]=[value]")
		}
		envMap[parts[0]] = parts[1]
	}
	return envMap, nil
}

func toProduct(dir string) string {
	switch dir {
	case "tas":
		return "ert"
	case "tasw":
		return "wrt"
	default:
		return dir
	}
}

// npmInstallCommand returns the appropriate npm install command.
// When not verbose, --silent suppresses all progress output; errors still cause
// a non-zero exit. When verbose, output is unrestricted so the user can see
// what npm is doing.
func npmInstallCommand(absoluteTileDir string, verbose bool) string {
	lockFile := filepath.Join(absoluteTileDir, "migrations", "package-lock.json")
	if _, err := os.Stat(lockFile); err == nil {
		if verbose {
			return "npm ci"
		}
		return "npm ci --silent"
	}
	if verbose {
		return "npm install --no-audit --no-fund"
	}
	return "npm install --no-audit --no-fund --silent"
}

func getTileTestEnvVars(dir, productDir string, envMap environmentVars) environmentVars {
	const fixturesFormat = "%s/test/manifest/fixtures"
	metadataPath := fmt.Sprintf(fixturesFormat+"/tas_metadata.yml", dir)
	configPath := fmt.Sprintf(fixturesFormat+"/tas_config.yml", dir)
	_, configErr := os.Stat(configPath)
	_, metadataErr := os.Stat(metadataPath)

	envVarsMap := make(map[string]string)
	if metadataErr == nil && configErr == nil {
		envVarsMap["TAS_METADATA_PATH"] = fmt.Sprintf(fixturesFormat+"/%s", "/tas/"+productDir, "tas_metadata.yml")
		envVarsMap["TAS_CONFIG_FILE"] = fmt.Sprintf(fixturesFormat+"/%s", "/tas/"+productDir, "tas_config.yml")
	}

	// no need to set for tas tile, since it defaults to ert.
	// for ist and tasw, we need to set it, as there's no default.
	if toProduct(productDir) != "ert" {
		envVarsMap["PRODUCT"] = toProduct(productDir)
	}
	envVarsMap["RENDERER"] = "ops-manifest"
	envVarsMap["GOMAXPROCS"] = strconv.Itoa(runtime.NumCPU())

	// overwrite with / include optional env vars
	for k, v := range envMap {
		envVarsMap[k] = v
	}
	return envVarsMap
}

//go:embed Dockerfile
var dockerfile string

type tarWriter interface {
	WriteHeader(hdr *tar.Header) error
	io.WriteCloser
}

type imageBuildMessage struct {
	Stream      string `json:"stream"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

// checkImageBuildResponse reads the Docker/Podman image-build JSON stream. If
// logOutput is non-nil, "stream" lines are copied there; otherwise they are
// discarded. Build failures are still returned from daemon "error" messages.
func checkImageBuildResponse(body io.ReadCloser, logOutput io.Writer) error {
	defer func() {
		_ = body.Close()
	}()
	decoder := json.NewDecoder(body)
	for {
		var msg imageBuildMessage
		if err := decoder.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read image build response: %w", err)
		}
		if logOutput != nil && msg.Stream != "" {
			_, _ = io.WriteString(logOutput, msg.Stream)
		}
		if msg.Error != "" {
			detail := msg.Error
			if msg.ErrorDetail.Message != "" {
				detail = msg.ErrorDetail.Message
			}
			return errors.New(detail)
		}
	}
	return nil
}

func createDockerfileTarball(tw tarWriter, fileContents string) error {
	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Mode: 0o600,
		Size: int64(len(fileContents)),
	}); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(fileContents)); err != nil {
		return err
	}
	return tw.Close()
}
