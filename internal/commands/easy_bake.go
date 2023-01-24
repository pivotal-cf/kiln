package commands

import (
<<<<<<< HEAD
	"context"
	"io"
	"log"
	"os"
=======
	"fmt"
	"log"
	"strings"
>>>>>>> f4818457 (wip: musings about kiln bake)

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
<<<<<<< HEAD
	logger  *log.Logger
	fetcher Fetch
=======
	logger          *log.Logger
	fs              billy.Filesystem
	releasesService baking.ReleasesService
	fetcher         Fetch
>>>>>>> f4818457 (wip: musings about kiln bake)
}

func NewEasyBake(logger *log.Logger, multiReleaseSourceProvider MultiReleaseSourceProvider, localReleaseDirectory LocalReleaseDirectory) EasyBake {
	fetcher := NewFetch(logger, multiReleaseSourceProvider, localReleaseDirectory)
	fetcher.Options.AllowOnlyPublishableReleases = true
	return EasyBake{
<<<<<<< HEAD
		logger:  logger,
		fetcher: fetcher,
=======
		logger:          logger,
		fs:              fs,
		releasesService: releaseService,
>>>>>>> f4818457 (wip: musings about kiln bake)
	}
}

var image = "gcr.io/tas-ppe/pivotalcfreleng/golang"

func (e EasyBake) Execute(args []string) error {
	e.logger.Println("We're fetching releases!")
	e.fetcher.Execute(args)

<<<<<<< HEAD
	e.logger.Println("We're baking a tile! (easy)")
	e.binBuild()
=======
	//download_releases "${tile_dir}"
	(func() { fmt.Println("downloading releases") })()
	//download_stemcell "${build_dir}"
	(func() { fmt.Println("downloading stemcell") })()
	//set_flags "${tile_dir}"
	tile_flags := ""
	explicitTile := fmt.Sprintf("-%s", args[0])
	switch args[0] {
	case "example-tile":
		explicitTile = ""
		tile_flags += "--migrations-directory=./migrations"
		args = append(args, tile_flags)
		args = args[1:]
	case "tas":
		tile_flags += "--migrations-directory ./migrations/ "
		tile_flags += "--migrations-directory ./migrations/${PRODUCT} "
		tile_flags += "--runtime-configs-directory ./runtime_configs "
		tile_flags += "--variables-file ./variables/${PRODUCT}.yml "
		args = append(args, tile_flags)
		args = args[1:]
	case "ist":
		tile_flags += "--migrations-directory ./migrations "
		tile_flags += "--variables-file ./variables/ist.yml "
		args = args[1:]
		args = append(args, tile_flags)
	case "windows":
		// figure out if it's hydrated
		// if not
		(func() { fmt.Println("hydration flag") })()
		tile_flags += "--migrations-directory ./migrations "
		tile_flags += "--variables-file ./variables/wrt.yml "
		args = args[1:]
		args = append(args, tile_flags)
		explicitTile = ""
	default:
	}

	args = append(args, "--metadata", "base.yml", "--variables-file", fmt.Sprintf("variables%.yml", explicitTile))
	fmt.Printf("running: bake %s\n", strings.Join(args, " "))
	err := NewBake(e.fs, e.releasesService, e.logger, e.logger).Execute(args)
>>>>>>> f4818457 (wip: musings about kiln bake)

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
