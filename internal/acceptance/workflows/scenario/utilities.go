package scenario

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"gopkg.in/yaml.v2"
)

var success error = nil

const (
	kilnDevVersion = "1.0.0-dev"

	indexNotFound = -1
)

func kilnCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "go", append([]string{"run", "-ldflags", "-X main.version=" + kilnDevVersion, "github.com/pivotal-cf/kiln", "--"}, args...)...)
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

func loadEnv(n string) (string, error) {
	v := os.Getenv(n)
	if v == "" {
		return "", fmt.Errorf("required env variable %s not set", n)
	}
	return v, success
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

func loadEnvVar(name, message string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("%s is not set (%s)", name, message)
	}
	return value, nil
}

func loadS3Credentials() (keyID, accessKey string, err error) {
	keyID, err = loadEnvVar("AWS_ACCESS_KEY_ID", "required for s3 release source to cache releases")
	if err != nil {
		return
	}
	accessKey, err = loadEnvVar("AWS_SECRET_ACCESS_KEY", "required for s3 release source to cache releases")
	if err != nil {
		return
	}
	return
}
