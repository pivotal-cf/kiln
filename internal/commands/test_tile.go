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

type status int64

func (statusState status) String() string {
	colorRed := "\033[31m"
	colorGreen := "\033[32m"

	if statusState == 0 {
		return fmt.Sprintf("%s%s", colorGreen, "success")
	} else {
		return fmt.Sprintf("%s%s (status code %d)", colorRed, "failure", statusState)
	}
}

// > one read per channel
//   (lexically: yes)

// option 1
// one go routine to read logs
//   for loop on logs, send to channel
// on main thread, read the above channel / interrupt
// see video

// option 2 (most simple)
// another way is to use signal with ctx. send to cnotainer ctx (signalwithctx)
// ctx.err should have info after we finish. can check if err = cancelled
// > keep track of the context tree so that cancels propagate to children correctly
// > one way cancel -> parent to child

// view chris' spike branch
//   -> result of it lead to dockerization due to local ruby setup (yes)

// use embed to ship assets
// docs (blackfriday), dockerfile?, etc
// nicer cli experience
// > try to improve presentation along with flow
// > consider hiding success and only logging failures

// try using go run ginkgo instead of ginko directly

func (u TestTile) Execute(args []string) error {
	u.logger.Printf("Beginning stability tests for tas tile")
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	defer func(cli *client.Client) {
		err := cli.Close()
		if err != nil {
			u.logger.Printf("error closing docker cli: +%v", err)
		}
	}(cli)

	if err != nil {
		return err
	}

	// todo: pull image

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: "gcr.io/tas-ppe/monorepo:25c68328471ce80cbcbf4dfe8045b754019e2e3b",
		Cmd:   []string{"/bin/bash", "-cx", "cd /tas/tas; for i in $(seq 1 3); do echo \"<log $i>\"; go test ./test/stability/; echo \"</log $i>\"; done; echo \"done\";"},
		Tty:   false,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: fmt.Sprintf("%s", "/Users/pivotal/workspace/tas/"),
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

	fmt.Printf("tests starting @ %s\n", time.Now())
	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	timeout := time.Minute * 1
	defer func(cli *client.Client, ctx context.Context, containerID string, timeout *time.Duration) {
		err := cli.ContainerStop(ctx, containerID, timeout)
		if err != nil {
			u.logger.Printf("error stopping container: +%v", err)
		}
	}(cli, ctx, resp.ID, &timeout)

	if err != nil {
		return err
	}
	bufOut := new(bytes.Buffer)
	w := bufio.NewWriter(bufOut)
	buf := make([]byte, 1024)
	//var readPointer int64 = 0
	f, _ := os.Create("/tmp/testyboi")
	f1, _ := os.OpenFile(f.Name(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)

	wf := bufio.NewWriter(f1)
readLogs:
	for {
		select {
		case <-interrupt:
			interrupt <- syscall.SIGINFO
			break readLogs
		default:
			n, err := io.ReadAtLeast(out, buf, 1)

			//io.NewSectionReader(out, readPointer, int64(n)-readPointer)
			if err != nil && err != io.EOF {
				interrupt <- syscall.SIGINT
				return err
			}
			if n == 0 {
				break readLogs
			}
			_, err = wf.Write([]byte(fmt.Sprintf("[%s] ", time.Now())))
			if err != nil {
				return err
			}
			_, err = w.Write([]byte(fmt.Sprintf("[%s] ", time.Now())))
			if err != nil {
				return err
			}
			if _, err := wf.Write(buf[:n]); err != nil {
				interrupt <- syscall.SIGINT
				return err
			}
			if _, err := w.Write(buf[:n]); err != nil {
				interrupt <- syscall.SIGINT
				return err
			}
			u.logger.Printf("line: %s", bufOut)
			err = w.Flush()
			_ = wf.Flush()
			if err != nil {
				return err
			}
		}
	}
	select {
	case x, ok := <-interrupt:
		if ok && x == syscall.SIGINFO {
			u.logger.Println("Cancelling tests!")
			return nil
		}
	default:
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	var statusCode status
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-statusCh:
	case testStatus := <-statusCh:
		statusCode = status(testStatus.StatusCode)
	}

	colorReset := "\033[0m"
	fmt.Printf("tests finished @ %s\ntest status %s%s!\n", time.Now(), statusCode, colorReset)
	return err
}

func (u TestTile) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "run tests spike",
		ShortDescription: "run tests",
		Flags:            u.Options,
	}
}
