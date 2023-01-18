package commands

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/pivotal-cf/jhanda"
	"io/ioutil"
	"log"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/kiln/internal/commands/flags"

	"github.com/docker/docker/client"
)

type TestTile struct {
	Options struct {
		flags.Standard
	}
	multiReleaseSourceProvider MultiReleaseSourceProvider
	filesystem                 billy.Filesystem
	logger                     *log.Logger
}

func NewTestTile(logger *log.Logger) TestTile {
	return TestTile{
		logger: logger,
	}
}

func (u TestTile) Execute(args []string) error {
	u.logger.Printf("Beginning stability tests for tas tile")
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: "gcr.io/tas-ppe/monorepo:latest",
		Cmd:   []string{"/bin/bash", "-c", "cd /tas/test/stability && go env -w GO111MODULE=off && for i in $(seq 1 10); do go test ./; done"},
		Tty:   false,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: fmt.Sprintf("%s", "/Users/pivotal/workspace/tas/tas"),
				Target: "/tas",
			},
		},
	}, nil, nil, "")
	if err != nil {
		return err
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	fmt.Printf("<wait condition WaitConditionNotRunning> %s\n", time.Now())
	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	var statusCode bool
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		statusCode = status.StatusCode != 1
	}

	fmt.Printf("</wait condition WaitConditionNotRunning> %s\n\n", time.Now())
	fmt.Printf("<container logs> %s\n", time.Now())
	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return err
	}
	fmt.Printf("</container logs> %s\n", time.Now())

	b, err := ioutil.ReadAll(out)
	if err != nil {
		return err
	}

	u.logger.Print(string(b))

	timeout := time.Minute * 1
	err = cli.ContainerStop(ctx, resp.ID, &timeout)

	u.logger.Printf("test success: %t\n", statusCode)

	return err

}

func (u TestTile) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "run tests spike",
		ShortDescription: "run tests",
		Flags:            u.Options,
	}
}
