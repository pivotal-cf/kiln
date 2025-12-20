package test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/pivotal-cf/kiln/internal/test/fakes"
	"github.com/stretchr/testify/require"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func TestConfiguration_commands(t *testing.T) {
	absoluteTileDirectory := filepath.Join(t.TempDir(), "test")
	require.NoError(t, os.MkdirAll(absoluteTileDirectory, 0o700))

	for _, tt := range []struct {
		Name            string
		Configuration   Configuration
		Result          []string
		ExpErrSubstring string
	}{
		{
			Name: "when the tile path is not absolute",
			Configuration: Configuration{
				AbsoluteTileDirectory: ".",
			},
			ExpErrSubstring: "tile path must be absolute",
		},
		{
			Name: "when no flags are true",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
			},
			Result: nil,
		},
		{
			Name: "when running migrations tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunMigrations:         true,
			},
			Result: []string{"cd /tas/test/migrations", "npm install", "npm test"},
		},
		{
			Name: "when running manifest tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunManifest:           true,
			},
			Result: []string{"cd /tas/test && ginkgo  /tas/test/test/manifest"},
		},
		{
			Name: "when running metadata tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunMetadata:           true,
			},
			Result: []string{"cd /tas/test && ginkgo  /tas/test/test/stability"},
		},
		{
			Name: "when running all tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunAll:                true,
			},
			Result: []string{"cd /tas/test/migrations", "npm install", "npm test", "cd /tas/test && ginkgo  /tas/test/test/stability /tas/test/test/manifest"},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			result, err := tt.Configuration.commands()
			if tt.ExpErrSubstring != "" {
				require.ErrorContains(t, err, tt.ExpErrSubstring)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.Result, result)
			}
		})
	}
}

func Test_configureSession(t *testing.T) {
	t.Run("when ping fails", func(t *testing.T) {
		ctx := context.Background()
		logger := log.New(io.Discard, "", 0)

		client := new(fakes.MobyClient)
		client.PingReturns(types.Ping{}, fmt.Errorf("lemon"))

		fn := func(string) error { panic("don't call this") }

		err := configureSession(ctx, logger, Configuration{}, client, fn)

		require.ErrorContains(t, err, "failed to connect to Docker daemon")
	})
}

func Test_runTestWithSession(t *testing.T) {
	absoluteTileDirectory := filepath.Join(t.TempDir(), "test")
	logger := log.New(io.Discard, "", 0)

	t.Run("when the command succeeds", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 0,
		})

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.NoError(t, err)
	})

	t.Run("when the command fails", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 22,
		})

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "test failed with exit code 22")
	})

	t.Run("when the command fails with an error message", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 22,
			Error: &container.WaitExitError{
				Message: "banana",
			},
		})
		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "test failed with exit code 22: banana")
	})

	t.Run("when fetching container logs fails", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 0,
		})
		client.ContainerLogsReturns(nil, fmt.Errorf("banana"))

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "container log request failure: ")
		require.ErrorContains(t, err, "banana")
	})

	t.Run("when starting the container container fails", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 0,
		})
		client.ContainerStartReturns(fmt.Errorf("banana"))

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "failed to start test container: ")
		require.ErrorContains(t, err, "banana")
	})
}

func runTestWithSessionHelper(t *testing.T, logs string, response container.WaitResponse) *fakes.MobyClient {
	t.Helper()
	client := new(fakes.MobyClient)
	client.ImageBuildReturns(build.ImageBuildResponse{
		Body: io.NopCloser(strings.NewReader("")),
	}, nil)
	client.ContainerStartReturns(nil)
	client.ContainerLogsReturns(io.NopCloser(strings.NewReader(logs)), nil)

	waitResp := make(chan container.WaitResponse)
	waitErr := make(chan error)
	client.ContainerWaitReturns(waitResp, waitErr)

	wg := sync.WaitGroup{}
	wg.Add(1)
	t.Cleanup(func() {
		wg.Wait()
	})
	testCtx, done := context.WithCancel(context.Background())
	go func() {
		defer wg.Done()
		select {
		case waitResp <- response:
		case <-testCtx.Done():
		}
	}()
	t.Cleanup(func() {
		done()
	})
	return client
}

func Test_decodeEnvironment(t *testing.T) {
	for _, tt := range []struct {
		Name            string
		In              []string
		Exp             map[string]string
		ExpErrSubstring string
	}{
		{
			Name: "valid variable",
			In:   []string{"fruit=orange"},
			Exp: map[string]string{
				"fruit": "orange",
			},
		},
		{
			Name:            "no separator",
			In:              []string{"fruit:orange"},
			ExpErrSubstring: "environment variables must have the format [key]=[value]",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := decodeEnvironment(tt.In)
			if tt.ExpErrSubstring != "" {
				require.ErrorContains(t, err, tt.ExpErrSubstring)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.Exp, got)
			}
		})
	}
}
