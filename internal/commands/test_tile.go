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

type ManifestTest struct {
	Options struct {
		TilePath            string `short:"tp"   long:"tile-path"                default:"."                          description:"path to the tile directory (e.g., ~/workspace/tas/ist)"`
		GingkoManifestFlags string `short:"gmf"  long:"ginkgo-manifest-flags"    default:"-r -slowSpecThreshold 15"   description:"flags to pass to the gingko manifest test suite"`
	}

	logger      *log.Logger
	ctx         context.Context
	cancelFunc  context.CancelFunc
	mobi        mobyClient
	sshProvider SshProvider
}

func NewManifestTest(logger *log.Logger, ctx context.Context, mobi mobyClient, sshThing SshProvider) ManifestTest {
	ctx, cancelFunc := context.WithCancel(ctx)
	return ManifestTest{
		ctx:         ctx,
		cancelFunc:  cancelFunc,
		logger:      logger,
		mobi:        mobi,
		sshProvider: sshThing,
	}
}

//go:embed manifest_test_docker/*
var dockerfileContents string

func (u ManifestTest) Execute(args []string) error {
	// TODO: check if ssh provider isn't borked
	if u.sshProvider == nil {
		return errors.New("ssh provider failed to initialize. check your ssh-agent is running")
	}
	_, err := jhanda.Parse(&u.Options, args)
	if err != nil {
		return fmt.Errorf("could not parse manifest-test flags: %s", err)
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

	u.logger.Println("Info: Checking for the latest ops-manager image...")

	tr, err := getTarReader(dockerfileContents)
	if err != nil {
		return err
	}

	u.logger.Println("Info: Building / restoring cached docker image. This may take several minutes during updates to Ops Manager or the first run...")
	res, err := u.mobi.ImageBuild(u.ctx, tr, types.ImageBuildOptions{
		Tags:      []string{"dont_push_me_vmware_confidential:123"},
		Version:   types.BuilderBuildKit,
		SessionID: session.ID(),
	})
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(res.Body)
	lastLine := ""
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
				Automatically looking in %s for ssh keys. We use SSH_AUTH_SOCK environment variable
                for the socket.`
				return fmt.Errorf(format, buildError, strings.Join(StandardSSHKeys, ", "))
			}
			return errors.New(buildError)
		}
		lastLine = text
	}
	u.logger.Println("Info:", lastLine)

	localTileDir := u.Options.TilePath
	absRepoDir, err := filepath.Abs(localTileDir)
	if err != nil {
		return err
	}
	parentDir := path.Dir(absRepoDir)
	tileDir := path.Base(absRepoDir)

	u.logger.Println("Info: Mounting ", parentDir, "and testing", tileDir)

	envVars := getManifestTestEnvVars(absRepoDir, tileDir)
	dockerCmd := fmt.Sprintf("cd /tas/%s/test/manifest && PRODUCT=%s RENDERER=ops-manifest ginkgo %s", tileDir, toProduct(tileDir), u.Options.GingkoManifestFlags)
	fmt.Println(dockerCmd)
	createResp, err := u.mobi.ContainerCreate(u.ctx, &container.Config{
		Image: "dont_push_me_vmware_confidential:123",
		Cmd:   []string{"/bin/bash", "-c", dockerCmd},
		Env:   envVars,
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
		fmt.Println("Canceling tests")
		err := u.mobi.ContainerStop(u.ctx, createResp.ID, container.StopOptions{
			Signal: "SIGKILL",
		})
		if err != nil {
			fmt.Println("Error stopping container", err)
		}
	}()

	out, err := u.mobi.ContainerLogs(u.ctx, createResp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return err
	}

	scanner = bufio.NewScanner(out)
	for scanner.Scan() {
		text := scanner.Text()
		u.logger.Println(text)
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

func getManifestTestEnvVars(dir, productDir string) []string {
	const fixturesFormat = "%s/test/manifest/fixtures"
	metadataPath := fmt.Sprintf(fixturesFormat+"/tas_metadata.yml", dir)
	configPath := fmt.Sprintf(fixturesFormat+"/tas_config.yml", dir)

	_, configErr := os.Stat(configPath)
	_, metadataErr := os.Stat(metadataPath)
	if metadataErr == nil && configErr == nil {
		return []string{
			fmt.Sprintf("TAS_METADATA_PATH=%s", fmt.Sprintf(fixturesFormat+"/%s", "/tas/"+productDir, "tas_metadata.yml")),
			fmt.Sprintf("TAS_CONFIG_FILE=%s", fmt.Sprintf(fixturesFormat+"/%s", "/tas/"+productDir, "tas_config.yml")),
		}
	}

	return nil
}

func getTarReader(fileContents string) (*bufio.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	tarHeader := &tar.Header{
		Name: "Dockerfile",
		Mode: 0600,
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

func (u ManifestTest) addMissingKeys() error {
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

type ErrorLine struct {
	Error string `json:"error"`
}

func (u ManifestTest) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Test the manifest for a product inside a docker container. Requires a docker daemon to be running and ssh keys with access to Ops Manager's git repo.",
		ShortDescription: "Test manifest for a product",
		Flags:            u.Options,
	}
}
