package commands

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/pivotal-cf/jhanda"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
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
	defer fmt.Println("kiln test exiting gracefully")

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: "gcr.io/tas-ppe/monorepo:latest",
		Cmd:   []string{"/bin/bash", "-c", "cd /tas/test/stability && go env -w GO111MODULE=off && for i in $(seq 1 3); do go test ./; sleep 1; done"},
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

	fmt.Printf("</wait condition WaitConditionNotRunning> %s\n\n", time.Now())
	fmt.Printf("<container logs> %s\n", time.Now())
	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	timeout := time.Minute * 1
	defer cli.ContainerStop(ctx, resp.ID, &timeout)

	if err != nil {
		return err
	}
	bufOut := new(bytes.Buffer)
	w := bufio.NewWriter(bufOut)
	buf := make([]byte, 1024)
	for {
		select {
		case interrupted := <-interrupt:
			fmt.Printf("got a ctrl-c %+v\n", interrupted)
			return nil
		default:
			n, err := io.ReadAtLeast(out, buf, 1)
			if err != nil && err != io.EOF {
				return err
			}
			if n == 0 {
				break
			}
			w.Write([]byte("line: "))
			if _, err := w.Write(buf[:n]); err != nil {
				return err
			}
			u.logger.Printf("line: %s", bufOut)
			w.Flush()
		}
	}

	fmt.Printf("</container logs> %s\n", time.Now())
	return err

}

func (u TestTile) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "run tests spike",
		ShortDescription: "run tests",
		Flags:            u.Options,
	}
}
