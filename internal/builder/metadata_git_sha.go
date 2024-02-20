package builder

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const DirtyWorktreeSHAValue = "DEVELOPMENT"

func GitMetadataSHA(repositoryDirectory string, isDev bool) (string, error) {
	if err := ensureGitExecutableIsFound(); err != nil {
		return "", err
	}
	if dirty, err := GitStateIsDirty(repositoryDirectory); err != nil {
		return "", err
	} else if dirty && isDev {
		_, _ = fmt.Fprintf(os.Stderr, "WARNING: git working directory has un-commited changes: the variable %q has has development only value %q", MetadataGitSHAVariable, DirtyWorktreeSHAValue)
		return DirtyWorktreeSHAValue, nil
	}
	return gitHeadRevision(repositoryDirectory)
}

func ModifiedTime(repositoryDirectory string, isDev bool) (time.Time, error) {
	if isDev {
		return time.Now(), nil
	}
	if err := ensureGitExecutableIsFound(); err != nil {
		return time.Time{}, err
	}
	if dirty, err := GitStateIsDirty(repositoryDirectory); err != nil {
		return time.Time{}, err
	} else if dirty && isDev {
		return time.Now(), nil
	}
	return GitCommitterCommitDate(repositoryDirectory)
}

func GitStateIsDirty(repositoryDirectory string) (bool, error) {
	gitStatus := exec.Command("git", "status", "--porcelain")
	gitStatus.Dir = repositoryDirectory
	err := gitStatus.Run()
	if err != nil {
		if gitStatus.ProcessState.ExitCode() == 1 {
			return true, nil
		}
		return true, fmt.Errorf("failed to run `%s %s`: %w", gitStatus.Path, strings.Join(gitStatus.Args, " "), err)
	}
	return false, nil
}

func GitCommitterCommitDate(repositoryDirectory string) (time.Time, error) {
	cmd := exec.Command("git", "show", "-s", "--format=%ct")
	cmd.Dir = repositoryDirectory
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, err
	}
	commitTime, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(commitTime, 0), nil
}

func gitHeadRevision(repositoryDirectory string) (string, error) {
	var out bytes.Buffer
	gitRevParseHead := exec.Command("git", "rev-parse", "HEAD")
	gitRevParseHead.Dir = repositoryDirectory
	gitRevParseHead.Stdout = &out
	err := gitRevParseHead.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD revision hash: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}

func ensureGitExecutableIsFound() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("could not calculate %q: %w", MetadataGitSHAVariable, err)
	}
	return nil
}
