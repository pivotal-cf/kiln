package steps

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
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
		cmd := exec.Command("gh", "auth", "status", "--show-token")
		var out bytes.Buffer
		cmd.Stderr = &out
		err := cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("login to github using the CLI or set GITHUB_TOKEN")
		}
		matches := regexp.MustCompile("(?m)^.*Token: (gho_.*)$").FindStringSubmatch(out.String())
		if len(matches) == 0 {
			return nil, fmt.Errorf("login to github using the CLI or set GITHUB_TOKEN")
		}
		token = matches[1]
	}
	return context.WithValue(ctx, githubTokenKey, token), nil
}

type Environment struct {
	OpsManagerPrivateKey string
	OpsManager           struct {
		URL      string
		Password string
		Username string
	}
	AvailabilityZones []string
	ServiceSubnetName string
}

func environment(ctx context.Context) (Environment, error) {
	return contextValue[Environment](ctx, environmentKey, "ops manager environment")
}

func loadEnvironment(ctx context.Context) (context.Context, error) {
	_, err := environment(ctx)
	if err == nil {
		return ctx, nil
	}

	var env Environment
	env.OpsManager.URL, err = loadEnv("OM_TARGET")
	if err != nil {
		return nil, err
	}
	env.OpsManager.Username, err = loadEnv("OM_USERNAME")
	if err != nil {
		return nil, err
	}
	env.OpsManager.Password, err = loadEnv("OM_PASSWORD")
	if err != nil {
		return nil, err
	}
	boshAllProxy, err := loadEnv("BOSH_ALL_PROXY")
	if err != nil {
		return nil, err
	}
	_, loadOmPrivateKeyErr := loadEnv("OM_PRIVATE_KEY")
	if loadOmPrivateKeyErr != nil {
		const failedToSetOMPrivateKey = "failed to set OM_PRIVATE_KEY from BOSH_ALL_PROXY: %w"
		u, err := url.Parse(boshAllProxy)
		if err != nil {
			return nil, fmt.Errorf(failedToSetOMPrivateKey, err)
		}
		keyPath := u.Query().Get("private-key")
		if keyPath == "" {
			return nil, fmt.Errorf(failedToSetOMPrivateKey, fmt.Errorf(`url did not have "private-key"`))
		}
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf(failedToSetOMPrivateKey, err)
		}
		err = os.Setenv("OM_PRIVATE_KEY", string(keyBytes))
		if err != nil {
			return nil, fmt.Errorf(failedToSetOMPrivateKey, err)
		}
	}

	var directorConfig struct {
		AvailabilityZoneConfiguration []struct {
			Name string `yaml:"name"`
		} `yaml:"az-configuration"`
		NetworkConfiguration struct {
			Networks []struct {
				Name string `yaml:"name"`
			} `yaml:"networks"`
		} `yaml:"networks-configuration"`
	}
	err = runAndParseStdoutAsYAML(
		exec.Command("om", "--skip-ssl-validation", "staged-director-config", "--no-redact"),
		&directorConfig,
	)
	if err != nil {
		return nil, err
	}

	for _, az := range directorConfig.AvailabilityZoneConfiguration {
		env.AvailabilityZones = append(env.AvailabilityZones, az.Name)
	}
	for _, network := range directorConfig.NetworkConfiguration.Networks {
		if !strings.HasSuffix(network.Name, "-services-subnet") {
			continue
		}
		env.ServiceSubnetName = network.Name
		break
	}
	return context.WithValue(ctx, environmentKey, env), nil
}

func publishableReleaseSource(ctx context.Context) (string, error) {
	return contextValue[string](ctx, publishableReleaseSourceKey, "publishable release source")
}

func setPublishableReleaseSource(ctx context.Context, e string) context.Context {
	return context.WithValue(ctx, publishableReleaseSourceKey, e)
}
