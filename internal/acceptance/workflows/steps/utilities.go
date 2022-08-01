package steps

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"gopkg.in/yaml.v2"
)

var success error = nil

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

func runAndParseStdoutAsYAML(cmd *exec.Cmd, d interface{}) error {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		_, _ = io.Copy(os.Stdout, &stderr)
		return err
	}
	return yaml.Unmarshal(stdout.Bytes(), d)
}

func runAndLogOnError(cmd *exec.Cmd) error {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		_, _ = io.Copy(os.Stdout, &buf)
	}
	return err
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
