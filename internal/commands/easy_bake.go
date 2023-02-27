package commands

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/commands/flags"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type EasyBake struct {
	Options struct {
		flags.Standard
	}
	logger  *log.Logger
	fetcher Fetch
}

func NewEasyBake(logger *log.Logger, multiReleaseSourceProvider MultiReleaseSourceProvider, localReleaseDirectory LocalReleaseDirectory) EasyBake {
	fetcher := NewFetch(logger, multiReleaseSourceProvider, localReleaseDirectory)
	fetcher.Options.AllowOnlyPublishableReleases = true
	return EasyBake{
		logger:  logger,
		fetcher: fetcher,
	}
}

var image = "gcr.io/tas-ppe/pivotalcfreleng/golang"

func (e EasyBake) Execute(args []string) error {
	e.logger.Println("We're fetching releases!")
	e.fetcher.Execute(args)

	e.logger.Println("We're baking a tile! (easy)")
	e.binBuild()

	return nil
}

func (e EasyBake) binBuild() {
	ctx := context.Background()
	runBuildRawInDocker(ctx)
}

func runBuildRawInDocker(ctx context.Context) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	pullImage(cli, ctx)

	c := createAndStartContainer(cli, ctx)

	waitForContainer(cli, ctx, c)

	processContainerLogs(cli, ctx, c)
}

func processContainerLogs(cli *client.Client, ctx context.Context, c container.ContainerCreateCreatedBody) {
	out, err := cli.ContainerLogs(ctx, c.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		panic(err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}

func waitForContainer(cli *client.Client, ctx context.Context, c container.ContainerCreateCreatedBody) {
	statusCh, errCh := cli.ContainerWait(ctx, c.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-statusCh:
	}
}

func createAndStartContainer(cli *client.Client, ctx context.Context) container.ContainerCreateCreatedBody {
	command := []string{"ls", "-la", "/tas"}

	config := &container.Config{
		Image: image,
		Cmd:   command,
		Tty:   false,
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: getTASRepo(),
				Target: "/tas",
			},
		},
	}

	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	return resp
}

func pullImage(cli *client.Client, ctx context.Context) {
	reader, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}

	defer reader.Close()
	io.Copy(os.Stdout, reader)
}

func getBinFolder() string {
	return "bin"
}

func getTASRepo() string {
	return "/Users/wadams/workspace/tas"
}

func (e EasyBake) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command is used to bake a release",
		ShortDescription: "bakes a release",
		Flags:            e.Options,
	}
}
