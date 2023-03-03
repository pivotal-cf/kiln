package builder

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const dirtyStateSHAValue = "DEVELOPMENT"

func GitMetadataSHA(repositoryDirectory string, isDev bool) func() (string, error) {
	var cache string
	return func() (s string, err error) {
		if cache != "" {
			return cache, nil
		}
		if _, err := exec.LookPath("git"); err != nil {
			return "", fmt.Errorf("could not calculate %q: %w", MetadataGitSHAVariable, err)
		}
		gitStatus := exec.Command("git", "status", "--porcelain")
		gitStatus.Dir = repositoryDirectory
		err = gitStatus.Run()
		if err != nil {
			if gitStatus.ProcessState.ExitCode() == 1 && isDev {
				_, _ = fmt.Fprintf(os.Stderr, "WARNING GIT state is not clean variable %q has value %q", MetadataGitSHAVariable, dirtyStateSHAValue)
				return "DEVELOPMENT", nil
			}
			return "", fmt.Errorf("failed to run git status: %w", err)
		}
		var out bytes.Buffer
		gitRevParseHead := exec.Command("git", "rev-parse", "HEAD")
		gitRevParseHead.Dir = repositoryDirectory
		gitRevParseHead.Stdout = &out
		err = gitRevParseHead.Run()
		if err != nil {
			return "", fmt.Errorf("failed to get HEAD revision hash: %w", err)
		}
		cache = strings.TrimSpace(out.String())
		return cache, nil
	}
}
