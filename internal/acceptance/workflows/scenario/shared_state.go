package scenario

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// key represents the type of the context key for shared values between steps
// see https://pkg.go.dev/context
type key int

const (
	tileRepoKey key = iota
	tileVersionKey
	githubTokenKey
	environmentKey
	publishableReleaseSourceKey
	standardFileDescriptorsKey
	lastCommandProcessStateKey
)

func contextValue[T any](ctx context.Context, k key, name string) (T, error) {
	s, ok := ctx.Value(k).(T)
	if !ok {
		var zeroValue T
		return zeroValue, fmt.Errorf("context value %s not set", name)
	}
	return s, nil
}

func tileRepoPath(ctx context.Context) (string, error) {
	return contextValue[string](ctx, tileRepoKey, "tile repository path")
}

func setTileRepoPath(ctx context.Context, p string) context.Context {
	return context.WithValue(ctx, tileRepoKey, p)
}

// defaultFilePathForTile returns a path based on the default output tile value of bake
func defaultFilePathForTile(ctx context.Context) (string, error) {
	p, err := tileRepoPath(ctx)
	if err != nil {
		return "", err
	}
	v, err := tileVersion(ctx)
	if err != nil {
		return "", err
	}
	result := filepath.Join(p, "tile-"+v+".pivotal")
	return result, nil
}

func kilnfilePath(ctx context.Context) (string, error) {
	p, err := tileRepoPath(ctx)
	if err != nil {
		return "", err
	}
	result := filepath.Join(p, "Kilnfile")
	return result, nil
}

func kilnfileLockPath(ctx context.Context) (string, error) {
	p, err := kilnfilePath(ctx)
	if err != nil {
		return "", err
	}
	result := p + ".lock"
	return result, nil
}

func tileVersion(ctx context.Context) (string, error) {
	return contextValue[string](ctx, tileVersionKey, "tile version")
}

func setTileVersion(ctx context.Context, p string) context.Context {
	return context.WithValue(ctx, tileVersionKey, p)
}

func githubToken(ctx context.Context) (string, error) {
	return contextValue[string](ctx, githubTokenKey, "github token")
}

func loadGithubToken(ctx context.Context) (context.Context, error) {
	_, err := githubToken(ctx)
	if err == nil {
		return ctx, nil
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token, err = getGithubTokenFromCLI()
		if err != nil {
			return ctx, err
		}
	}
	return context.WithValue(ctx, githubTokenKey, token), nil
}

func environment(ctx context.Context) (opsManagerEnvironment, error) {
	return contextValue[opsManagerEnvironment](ctx, environmentKey, "ops manager environment")
}

func loadEnvironment(ctx context.Context) (context.Context, error) {
	_, err := environment(ctx)
	if err == nil {
		return ctx, nil
	}

	var omEnv opsManagerEnvironment
	err = omEnv.loadFromEnvironmentVariables()
	if err != nil {
		return ctx, err
	}
	err = omEnv.fetchNetworksAndAvailabilityZones(ctx)
	if err != nil {
		return ctx, err
	}

	return context.WithValue(ctx, environmentKey, omEnv), nil
}

func publishableReleaseSource(ctx context.Context) (string, error) {
	return contextValue[string](ctx, publishableReleaseSourceKey, "publishable release source")
}

func setPublishableReleaseSource(ctx context.Context, e string) context.Context {
	return context.WithValue(ctx, publishableReleaseSourceKey, e)
}

type standardFileDescriptors [3]*bytes.Buffer

func output(ctx context.Context, name string) (*bytes.Buffer, error) {
	v, err := contextValue[standardFileDescriptors](ctx, standardFileDescriptorsKey, name)
	if err != nil {
		return nil, err
	}
	switch name {
	case "stdout":
		return v[1], nil
	case "stderr":
		return v[2], nil
	default:
		name, err = strconv.Unquote(name)
		if err != nil {
			return nil, err
		}
		buf, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(buf), nil
	}
}

func configureStandardFileDescriptors(ctx context.Context) context.Context {
	outputs := standardFileDescriptors{
		nil, // stdin is not yet implemented
		bytes.NewBuffer(nil),
		bytes.NewBuffer(nil),
	}
	return context.WithValue(ctx, standardFileDescriptorsKey, outputs)
}

func lastCommandProcessState(ctx context.Context) (*os.ProcessState, error) {
	return contextValue[*os.ProcessState](ctx, lastCommandProcessStateKey, "last command process state")
}

func setLastCommandStatus(ctx context.Context, state *os.ProcessState) context.Context {
	return context.WithValue(ctx, lastCommandProcessStateKey, state)
}