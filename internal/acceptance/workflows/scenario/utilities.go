package scenario

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	indexNotFound = -1

	kilnBuildCacheKey kilnBuildCacheKeyType = 0
)

type kilnBuildCacheKeyType int

func kilnCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, kilnBuildPath(ctx), args...)
}

func executeAndWrapError(wd string, env []string, command string, args ...string) error {
	var output bytes.Buffer
	cmd := exec.Command(command, args...)

	if env == nil {
		env = cmd.Environ()
	}

	cmd.Dir = wd
	cmd.Env = env
	cmd.Stderr = &output
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("executing `%s` failed with error %w\n%s", strings.Join(cmd.Args, " "), err, output.String())
	}
	return nil
}

func closeAndIgnoreErr(c io.Closer) {
	_ = c.Close()
}

func loadFileAsYAML(filePath string, v any) error {
	kfBuf, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(kfBuf, v)
	if err != nil {
		return fmt.Errorf("failed to parse kilnfile: %w", err)
	}
	return nil
}

func saveAsYAML(filePath string, v any) error {
	kfBuf, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to render kilnfile: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer closeAndIgnoreErr(f)

	_, err = f.Write(kfBuf)
	return err
}

func getGithubTokenFromCLI() (string, error) {
	cmd := exec.Command("gh", "auth", "status", "--show-token")
	var out bytes.Buffer
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("login to github using the CLI or set GITHUB_TOKEN")
	}
	matches := regexp.MustCompile("(?m)^.*Token: (gho_.*)$").FindStringSubmatch(out.String())
	if len(matches) == 0 {
		return "", fmt.Errorf("login to github using the CLI or set GITHUB_TOKEN")
	}
	return matches[1], nil
}
