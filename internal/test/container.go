package test

import (
	"archive/tar"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/moby/buildkit/session"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

const (
	// MinimumDockerServerVersion the test was failing with an older version this may be a bit conservative.
	// If the integration tests pass on your machine with an older version, feel free to PR a less conservative value.
	MinimumDockerServerVersion = "> 24.0.0"
	MinimumPodmanServerVersion = "> 5.3.0"
)

func Run(ctx context.Context, w io.Writer, configuration Configuration) error {
	logger := log.New(w, "kiln test: ", log.Default().Flags())

	dockerDaemon, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	return configureSession(ctx, logger, configuration, dockerDaemon, runTestWithSession(ctx, logger, w, dockerDaemon, configuration))
}

type Configuration struct {
	SSHSocketAddress string

	AbsoluteTileDirectory string

	RunAll,
	RunMigrations,
	RunManifest,
	RunMetadata bool

	GinkgoFlags string
	Environment []string
}

func (configuration Configuration) commands() ([]string, error) {
	if !filepath.IsAbs(configuration.AbsoluteTileDirectory) {
		return nil, fmt.Errorf("tile path must be absolute")
	}
	tileDirName := filepath.Base(configuration.AbsoluteTileDirectory)

	commands := []string{"git config --global --add safe.directory '*'"}
	if configuration.RunMigrations || configuration.RunAll {
		commands = append(commands, fmt.Sprintf("cd /tas/%s/migrations", tileDirName))
		commands = append(commands, "npm install")
		commands = append(commands, "npm test")
	}
	var ginkgo []string
	if configuration.RunMetadata || configuration.RunAll {
		ginkgo = append(ginkgo, fmt.Sprintf("/tas/%s/test/stability", tileDirName))
	}
	if configuration.RunManifest || configuration.RunAll {
		ginkgo = append(ginkgo, fmt.Sprintf("/tas/%s/test/manifest", tileDirName))
	}
	if configuration.RunMetadata || configuration.RunManifest || configuration.RunAll {
		ginkgoCommand := fmt.Sprintf("cd /tas && go install github.com/onsi/ginkgo/ginkgo@latest && cd %s && ginkgo -timeout=1h %s %s", tileDirName, configuration.GinkgoFlags, strings.Join(ginkgo, " "))
		commands = append(commands, ginkgoCommand)
	}
	return commands, nil
}

//counterfeiter:generate -o ./fakes/moby_client.go --fake-name MobyClient . mobyClient
type mobyClient interface {
	DialHijack(ctx context.Context, url, proto string, meta map[string][]string) (net.Conn, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (build.ImageBuildResponse, error)
	Ping(ctx context.Context) (types.Ping, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specV1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
}

func runTestWithSession(ctx context.Context, logger *log.Logger, w io.Writer, dockerDaemon mobyClient, configuration Configuration) func(sessionID string) error {
	return func(sessionID string) error {
		commands, err := configuration.commands()
		if err != nil {
			return err
		}

		var dockerfileTarball bytes.Buffer
		if err := createDockerfileTarball(tar.NewWriter(&dockerfileTarball), dockerfile); err != nil {
			return err
		}

		envMap, err := decodeEnvironment(configuration.Environment)
		if err != nil {
			return fmt.Errorf("failed to parse environment: %w", err)
		}

		artifactoryUsername := envMap["ARTIFACTORY_USERNAME"]
		artifactoryPassword := envMap["ARTIFACTORY_PASSWORD"]

		logger.Println("creating test image")
		resp, err := dockerDaemon.ImageBuild(ctx, &dockerfileTarball, build.ImageBuildOptions{
			Tags:      []string{"kiln_test_dependencies:vmware"},
			Version:   build.BuilderBuildKit,
			SessionID: sessionID,
			BuildArgs: map[string]*string{
				"ARTIFACTORY_USERNAME": &artifactoryUsername,
				"ARTIFACTORY_PASSWORD": &artifactoryPassword,
			},
		})

		if err != nil {
			return fmt.Errorf("failed to build image: %w", err)
		}

		logger.Println("reading image build response")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read image build response: %w", err)
		}
		fmt.Println(string(body))

		parentDir := path.Dir(configuration.AbsoluteTileDirectory)
		tileDir := path.Base(configuration.AbsoluteTileDirectory)

		dockerCmd := strings.Join(commands, " && ")

		envVars := getTileTestEnvVars(configuration.AbsoluteTileDirectory, tileDir, envMap)
		logger.Println("creating test container")
		testContainer, err := dockerDaemon.ContainerCreate(ctx, &container.Config{
			Image: "kiln_test_dependencies:vmware",
			Cmd:   []string{"/bin/bash", "-c", dockerCmd},
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
		logger.Printf("created test container with id %s", testContainer.ID)

		errG := errgroup.Group{}

		sigInt := make(chan os.Signal, 1)
		signal.Notify(sigInt, os.Interrupt)
		errG.Go(func() error {
			<-sigInt
			err := dockerDaemon.ContainerStop(ctx, testContainer.ID, container.StopOptions{
				Signal: "SIGKILL",
			})
			if err != nil {
				return fmt.Errorf("failed to stop container: %w", err)
			}
			return nil
		})

		if err := dockerDaemon.ContainerStart(ctx, testContainer.ID, container.StartOptions{}); err != nil {
			return fmt.Errorf("failed to start test container: %w", err)
		}

		out, err := dockerDaemon.ContainerLogs(ctx, testContainer.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
		if err != nil {
			return fmt.Errorf("container log request failure: %w", err)
		}
		if _, err := io.Copy(w, out); err != nil {
			return err
		}

		// Although the fan-in loop pattern seems like the right solution here, ContainerWait
		// does not properly close channels, so it won't work.
		var resultErr error
		statusCh, containerWaitError := dockerDaemon.ContainerWait(ctx, testContainer.ID, container.WaitConditionNotRunning)
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
}

// configureSession is the part of the code that sets up socket connections and interacts with the daemon
// testing it is non-trivial, so I isolated it. Testing it properly would require a daemon connection.
func configureSession(ctx context.Context, logger *log.Logger, configuration Configuration, dockerDaemon mobyClient, function func(sessionID string) error) error {
	logger.Printf("pinging docker daemon")
	_, err := dockerDaemon.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	s, err := session.NewSession(ctx, "waypoint")
	if err != nil {
		return fmt.Errorf("failed to create docker daemon session: %w", err)
	}
	defer closeAndIgnoreError(s)

	runErrC := make(chan error)
	go func() {
		defer close(runErrC)
		runErrC <- s.Run(ctx, func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
			conn, err := dockerDaemon.DialHijack(ctx, "/session", proto, meta)
			if err != nil {
				return nil, fmt.Errorf("session hijack error: %w", err)
			}
			return conn, nil
		})
	}()

	logger.Println("completed session setup")

	err = function(s.ID())
	_ = s.Close()
	for e := range runErrC {
		err = errors.Join(err, e)
	}
	return err
}

type environmentVars = map[string]string

func encodeEnvironment(m environmentVars) []string {
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, strings.Join([]string{k, v}, "="))
	}
	return result
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

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}
