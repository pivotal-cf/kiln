package scenario

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"gopkg.in/yaml.v2"
)

const (
	indexNotFound = -1

	kilnBuildCacheKey kilnBuildCacheKeyType = 0
)

type kilnBuildCacheKeyType int

func kilnCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, kilnBuildPath(ctx), args...)
}

func checkoutMain(repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"),
		Force:  true,
	})
	if err != nil {
		return err
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
