package steps

import (
	"context"
	"os/exec"
)

// iInvokeKilnBake invokes kiln bake with tileVersion provided by iHaveARepositoryCheckedOutAtRevision
func iInvokeKilnBake(ctx context.Context) error {
	version, err := tileVersion(ctx)
	if err != nil {
		return err
	}
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "github.com/pivotal-cf/kiln", "bake", "--version", version)
	cmd.Dir = repoPath
	return runAndLogOnError(cmd)
}
