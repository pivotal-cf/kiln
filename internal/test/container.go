package test

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/homedir"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/sshforward"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/exp/slices"
	"golang.org/x/term"
)

const (
	authSockEnvVarName    = "SSH_AUTH_SOCK"
	sshPasswordEnvVarName = "SSH_PASSWORD"
)

func Run(ctx context.Context, w io.Writer, configuration Configuration) error {
	logger := log.New(w, "kiln test: ", log.Default().Flags())
	if found := configuration.SSHSocketAddress != ""; !found {
		configuration.SSHSocketAddress, found = os.LookupEnv(authSockEnvVarName)
		if !found {
			return fmt.Errorf("neither configuration.SSHSocketAddress nor environment variable %s are set", authSockEnvVarName)
		}
	}

	dockerDaemon, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	logger.Printf("connecting to ssh socket %q", configuration.SSHSocketAddress)
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", configuration.SSHSocketAddress)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %w", authSockEnvVarName, err)
	}

	home := homedir.Get()

	logger.Printf("ensuring ssh agent keys are configured")
	if err := ensureSSHAgentKeys(agent.NewClient(conn), home, keyPasswordService{
		stdin:        os.Stdin,
		stdout:       os.Stdout,
		isTerm:       term.IsTerminal,
		readPassword: term.ReadPassword,
	}.password); err != nil {
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

	var commands []string
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
		ginkgoCommand := fmt.Sprintf("cd /tas/%s && ginkgo %s %s", tileDirName, configuration.GinkgoFlags, strings.Join(ginkgo, " "))
		commands = append(commands, ginkgoCommand)
	}
	return commands, nil
}

//counterfeiter:generate -o ./fakes/moby_client.go --fake-name MobyClient . mobyClient
type mobyClient interface {
	DialHijack(ctx context.Context, url, proto string, meta map[string][]string) (net.Conn, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	Ping(ctx context.Context) (types.Ping, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specV1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)
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

		logger.Println("creating test image")
		imageBuildResult, err := dockerDaemon.ImageBuild(ctx, &dockerfileTarball, types.ImageBuildOptions{
			Tags:      []string{"kiln_test_dependencies:vmware"},
			Version:   types.BuilderBuildKit,
			SessionID: sessionID,
		})
		if err != nil {
			return fmt.Errorf("failed to build image: %w", err)
		}

		if err := checkSSHPrivateKeyError(imageBuildResult.Body); err != nil {
			return err
		}

		parentDir := path.Dir(configuration.AbsoluteTileDirectory)
		tileDir := path.Base(configuration.AbsoluteTileDirectory)

		envMap, err := decodeEnvironment(configuration.Environment)
		if err != nil {
			return fmt.Errorf("failed to parse environment: %s", err)
		}

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

		errG, ctx := errgroup.WithContext(ctx)

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

		if err := dockerDaemon.ContainerStart(ctx, testContainer.ID, types.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("failed to start test container: %w", err)
		}

		out, err := dockerDaemon.ContainerLogs(ctx, testContainer.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
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

	if _, err := dockerDaemon.Ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	s, err := session.NewSession(ctx, "waypoint", "")
	if err != nil {
		return fmt.Errorf("failed to create docker daemon session: %w", err)
	}
	defer closeAndIgnoreError(s)

	sshProvider, err := sshprovider.NewSSHAgentProvider([]sshprovider.AgentConfig{{ID: sshforward.DefaultID, Paths: []string{configuration.SSHSocketAddress}}})
	if err != nil {
		return fmt.Errorf("failed to initalize ssh-agent provider: %w", err)
	}
	s.Allow(sshProvider)

	runErrC := make(chan error)
	go func() {
		defer close(runErrC)
		runErrC <- s.Run(ctx, func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
			conn, err := dockerDaemon.DialHijack(ctx, "/session", proto, meta)
			if err != nil {
				return nil, fmt.Errorf("session hyjack error: %w", err)
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

func checkSSHPrivateKeyError(buildResult io.Reader) error {
	type errorLine struct {
		Error string `json:"error"`
	}

	scanner := bufio.NewScanner(buildResult)
	for scanner.Scan() {
		text := scanner.Text()
		var line errorLine
		err := json.Unmarshal([]byte(text), &line)
		if err != nil {
			return fmt.Errorf("error unmarshalling json: %s", text)
		}
		if line.Error != "" {
			if strings.Contains(line.Error, "exit code: 128") {
				format := `does your private key have access to the ops-manifest repo?\n error: %s
				automatically looking in %s for ssh keys. SSH_AUTH_SOCK needs to be set.`
				return fmt.Errorf(format, line.Error, strings.Join(standardSSHKeyFileBases(), ", "))
			}
			return errors.New(line.Error)
		}
	}
	return nil
}

//counterfeiter:generate -o ./fakes/ssh_agent.go --fake-name SSHAgent . sshAgent
type sshAgent interface {
	Add(key agent.AddedKey) error
	List() ([]*agent.Key, error)
}

func ensureSSHAgentKeys(agent sshAgent, homeDirectory string, password func() ([]byte, error)) error {
	keys, err := agent.List()
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}
	if len(keys) > 0 {
		return nil
	}
	return loadDefaultKeys(agent, homeDirectory, password)
}

func standardSSHKeyFileBases() []string {
	return slices.Clone([]string{
		"id_rsa",
		"id_dsa",
		"id_ecdsa",
		"id_ed25519",
		"identity",
	})
}

func loadDefaultKeys(agent sshAgent, home string, password func() ([]byte, error)) error {
	for _, keyName := range standardSSHKeyFileBases() {
		keyFilePath := filepath.Join(home, ".ssh", keyName)
		_, err := os.Stat(keyFilePath)
		if err != nil {
			continue
		}
		err = addKey(agent, keyFilePath, password)
		if err != nil {
			return fmt.Errorf("failed to read key %s: %w", filepath.Base(keyFilePath), err)
		}
		break
	}
	return nil
}

func addKey(a sshAgent, keyFilePath string, password func() ([]byte, error)) error {
	keyFileContents, err := os.ReadFile(keyFilePath)
	if err != nil {
		return err
	}
	decryptedKey, err := ssh.ParseRawPrivateKey(keyFileContents)
	if err != nil {
		passphraseMissingError := new(ssh.PassphraseMissingError)
		if !errors.As(err, &passphraseMissingError) {
			return err
		}
		ps, err := password()
		if err != nil {
			return fmt.Errorf("failed to get password: %w", err)
		}
		decryptedKey, err = ssh.ParseRawPrivateKeyWithPassphrase(keyFileContents, ps)
		if err != nil {
			return err
		}
	}
	return a.Add(agent.AddedKey{PrivateKey: decryptedKey})
}

type keyPasswordService struct {
	stdout       io.Writer
	stdin        *os.File
	isTerm       func(int) bool
	readPassword func(int) ([]byte, error)
}

func (in keyPasswordService) password() ([]byte, error) {
	if password, found := os.LookupEnv(sshPasswordEnvVarName); found {
		return []byte(password), nil
	}
	return in.readPasswordFromStdin()
}

func (in keyPasswordService) readPasswordFromStdin() ([]byte, error) {
	if in.isTerm(int(in.stdin.Fd())) {
		_, _ = io.WriteString(in.stdout, "Enter password: ")
		buf, err := in.readPassword(int(in.stdin.Fd()))
		if err != nil {
			return nil, fmt.Errorf("failed to read password: %w", err)
		}
		return buf, nil
	}
	buf, err := bufio.NewReader(in.stdin).ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read from standard input: %w", err)
	}
	buf = bytes.TrimRight(buf, "\n")
	return buf, nil
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
