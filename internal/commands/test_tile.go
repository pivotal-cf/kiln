package commands

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"

	mobySession "github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pivotal-cf/jhanda"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

//counterfeiter:generate -o ./fakes/moby_client.go --fake-name MobyClient . mobyClient
type mobyClient interface {
	DialHijack(ctx context.Context, url, proto string, meta map[string][]string) (net.Conn, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	Ping(ctx context.Context) (types.Ping, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
}

type infoLog struct {
	logger  *log.Logger
	logging *sync.Mutex
	enabled bool
}

func (l infoLog) Info(v ...any) {
	if l.enabled {
		// the below should be a single atomic op
		l.logging.Lock()
		defer l.logging.Unlock()
		originalFlags := l.logger.Flags()
		originalPrefix := l.logger.Prefix()
		l.logger.SetFlags(log.Lmicroseconds)
		l.logger.SetPrefix("Info: ")
		l.logger.Println(v...)
		l.logger.SetFlags(originalFlags)
		l.logger.SetPrefix(originalPrefix)
	}
}

func (l infoLog) Println(v ...any) {
	l.logger.Println(v...)
}

func (l infoLog) Writer() io.Writer {
	return l.logger.Writer()
}

type TileTest struct {
	Options struct {
		TilePath        string   `short:"tp"   long:"tile-path"                default:"."                             description:"Path to the Tile directory (e.g., ~/workspace/tas/ist)."`
		GingkoFlags     string   `short:"gmf"  long:"ginkgo-flags"             default:"-r -p -slowSpecThreshold 15"   description:"Flags to pass to the Ginkgo Manifest and Stability test suites."`
		Verbose         bool     `short:"v"    long:"verbose"                  default:"false"                         description:"Print info lines. This doesn't affect Ginkgo output."`
		Manifest        bool     `             long:"manifest"                 default:"false"                         description:"Focus the Manifest tests."`
		Migrations      bool     `             long:"migrations"               default:"false"                         description:"Focus the Migration tests."`
		Stability       bool     `             long:"stability"                default:"false"                         description:"Focus the Stability tests."`
		EnvironmentVars []string `short:"e"    long:"environment-variable"                                             description:"Pass environment variable to the test suites. For example --stability -e 'PRODUCT=srt'."`
	}

	logger      *log.Logger
	ctx         context.Context
	cancelFunc  context.CancelFunc
	mobi        mobyClient
	sshProvider SshProvider
}

func NewTileTest(logger *log.Logger, ctx context.Context, mobi mobyClient, sshThing SshProvider) TileTest {
	ctx, cancelFunc := context.WithCancel(ctx)
	return TileTest{
		ctx:         ctx,
		cancelFunc:  cancelFunc,
		logger:      logger,
		mobi:        mobi,
		sshProvider: sshThing,
	}
}

//go:embed manifest_test_docker/*
var dockerfileContents string

func (u TileTest) Execute(args []string) error {
	if u.sshProvider == nil {
		return errors.New("ssh provider failed to initialize. check your ssh-agent is running")
	}
	_, err := jhanda.Parse(&u.Options, args)
	if err != nil {
		return fmt.Errorf("could not parse manifest-test flags: %s", err)
	}
	envMap, err := validateAndParseEnvVars(u.Options.EnvironmentVars)
	if err != nil {
		return fmt.Errorf("could not parse manifest-test flags: %s", err)
	}

	loggerWithInfo := infoLog{
		logger:  u.logger,
		logging: &sync.Mutex{},
		enabled: u.Options.Verbose,
	}

	_, err = u.mobi.Ping(u.ctx)
	if err != nil {
		return errors.New("Docker daemon is not running")
	}

	err = u.addMissingKeys()
	if err != nil {
		return err
	}
	sshp, err := sshprovider.NewSSHAgentProvider([]sshprovider.AgentConfig{{ID: "default", Paths: nil}})
	if err != nil {
		return err
	}

	session, _ := mobySession.NewSession(u.ctx, "waypoint", "")
	defer closeAndIgnoreError(session)
	session.Allow(sshp)
	dialSession := func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
		return u.mobi.DialHijack(ctx, "/session", proto, meta)
	}
	go func() {
		err := session.Run(u.ctx, dialSession)
		if err != nil {
			fmt.Printf("%+v\n", err)
		}
	}()

	loggerWithInfo.Info("Checking for the latest ops-manager image...")

	tr, err := getTarReader(dockerfileContents)
	if err != nil {
		return err
	}

	loggerWithInfo.Info("Building / restoring cached docker image")
	loggerWithInfo.Info("This may take several minutes during updates to Ops Manager or the first run...")
	res, err := u.mobi.ImageBuild(u.ctx, tr, types.ImageBuildOptions{
		Tags:      []string{"kiln_test_dependencies:vmware"},
		Version:   types.BuilderBuildKit,
		SessionID: session.ID(),
	})
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		text := scanner.Text()
		errLine := &ErrorLine{}
		err := json.Unmarshal([]byte(text), errLine)
		if err != nil {
			return fmt.Errorf("error unmarshalling json: %s", text)
		}
		buildError := errLine.Error
		if buildError != "" {
			if strings.Contains(buildError, "exit code: 128") {
				format := `Does your private key have access to the ops-manifest repo?\n error: %s
				Automatically looking in %s for ssh keys. SSH_AUTH_SOCK needs to be set.`
				return fmt.Errorf(format, buildError, strings.Join(StandardSSHKeys, ", "))
			}
			return errors.New(buildError)
		}
	}

	localTileDir := u.Options.TilePath
	absRepoDir, err := filepath.Abs(localTileDir)
	if err != nil {
		return err
	}
	parentDir := path.Dir(absRepoDir)
	tileDir := path.Base(absRepoDir)

	loggerWithInfo.Info("Mounting", parentDir, "and testing", tileDir)

	runAll := !u.Options.Manifest && !u.Options.Migrations && !u.Options.Stability

	var dockerCmds []string
	if u.Options.Migrations || runAll {
		dockerCmds = append(dockerCmds, fmt.Sprintf("cd /tas/%s/migrations", tileDir))
		dockerCmds = append(dockerCmds, "npm install")
		dockerCmds = append(dockerCmds, "npm test")
	}
	ginkgo := []string{}
	if u.Options.Stability || runAll {
		ginkgo = append(ginkgo, fmt.Sprintf("/tas/%s/test/stability", tileDir))
	}
	if u.Options.Manifest || runAll {
		ginkgo = append(ginkgo, fmt.Sprintf("/tas/%s/test/manifest", tileDir))
	}
	if u.Options.Stability || u.Options.Manifest || runAll {
		ginkgoCommand := fmt.Sprintf("cd /tas/%s && ginkgo %s %s", tileDir, u.Options.GingkoFlags, strings.Join(ginkgo, " "))
		dockerCmds = append(dockerCmds, ginkgoCommand)
	}

	dockerCmd := strings.Join(dockerCmds, " && ")
	loggerWithInfo.Info("Running:", dockerCmd)
	envVars := getTileTestEnvVars(absRepoDir, tileDir, envMap)
	createResp, err := u.mobi.ContainerCreate(u.ctx, &container.Config{
		Image: "kiln_test_dependencies:vmware",
		Cmd:   []string{"/bin/bash", "-c", dockerCmd},
		Env:   envVarsToSlice(envVars),
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
		return err
	}
	sigInt := make(chan struct{})
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		close(sigInt)
	}()

	if err := u.mobi.ContainerStart(u.ctx, createResp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	go func() {
		<-sigInt
		loggerWithInfo.Println("Canceling tests")
		err := u.mobi.ContainerStop(u.ctx, createResp.ID, container.StopOptions{
			Signal: "SIGKILL",
		})
		if err != nil {
			loggerWithInfo.Println("Error stopping container", err)
		}
	}()

	out, err := u.mobi.ContainerLogs(u.ctx, createResp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return err
	}

	_, err = io.Copy(loggerWithInfo.Writer(), out)
	if err != nil {
		return err
	}

	statusCh, errCh := u.mobi.ContainerWait(u.ctx, createResp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return err
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return errors.New(fmt.Sprintf("%+v", status))
		}
	}

	return nil
}

func validateAndParseEnvVars(environmentVarArgs []string) (environmentVars, error) {
	envMap := make(environmentVars)
	for _, envVar := range environmentVarArgs {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("Environment variables must have the format [key]=[value]")
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

func getTarReader(fileContents string) (*bufio.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	tarHeader := &tar.Header{
		Name: "Dockerfile",
		Mode: 0o600,
		Size: int64(len(fileContents)),
	}
	err := tw.WriteHeader(tarHeader)
	if err != nil {
		return nil, err
	}
	_, err = tw.Write([]byte(fileContents))
	if err != nil {
		return nil, err
	}
	err = tw.Close()
	if err != nil {
		return nil, err
	}
	tr := bufio.NewReader(&buf)
	return tr, nil
}

func (u TileTest) addMissingKeys() error {
	needsKeys, err := u.sshProvider.NeedsKeys()
	if needsKeys {
		key, err := u.sshProvider.GetKeys()
		if err != nil {
			return err
		}
		var bytePassword []byte
		if key.Encrypted {
			switch {
			// for non-interactive use, use SSH_PASSWORD env var
			case os.Getenv("SSH_PASSWORD") != "":
				bytePassword = []byte(os.Getenv("SSH_PASSWORD"))
			case term.IsTerminal(int(os.Stdin.Fd())):
				fmt.Printf("Enter password: ")
				bytePassword, err = term.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return errors.Wrapf(err, "failed to read %s", key.KeyPath)
				}
			default:
				reader := bufio.NewReader(os.Stdin)
				bytePassword, err = reader.ReadBytes('\n')
				bytePassword = bytes.TrimRight(bytePassword, "\n")
				if err != nil {
					log.Fatalf("Error when entering password: %s", err.Error())
				}
			}
			u.logger.Println()
		}
		err = u.sshProvider.AddKey(key, bytePassword)
		if err != nil {
			log.Fatalf("Failed to add key: %s", err)
		}
	}
	return err
}

type environmentVars map[string]string

func envVarsToSlice(envVars environmentVars) []string {
	convertedEnvVars := []string{}
	for k, v := range envVars {
		convertedEnvVars = append(convertedEnvVars, fmt.Sprintf("%s=%s", k, v))
	}
	return convertedEnvVars
}

type ErrorLine struct {
	Error string `json:"error"`
}

func (u TileTest) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Run the Manifest, Migrations, and Stability tests for a Tile in a Docker container. Requires a Docker daemon to be running and ssh keys with access to Ops Manager's Git repository. For non-interactive use, either set the environment variable SSH_PASSWORD, or `ssh add` your identity before running.",
		ShortDescription: "Runs unit tests for a Tile.",
		Flags:            u.Options,
	}
}
