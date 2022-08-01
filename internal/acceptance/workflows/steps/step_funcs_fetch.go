package steps

import (
	"context"
	"os/exec"

	"github.com/cucumber/godog"
)

// iInvokeKilnFetch fetches releases. It provides the command with the GitHub token (used for hello-release).
func iInvokeKilnFetch(ctx context.Context) error {
	token, err := githubToken(ctx)
	if err != nil {
		return err
	}
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "github.com/pivotal-cf/kiln", "fetch", "--no-confirm", "--variable", "github_token="+token)
	cmd.Dir = repoPath
	return runAndLogOnError(cmd)
}

// cleanUpFetchedReleases should be run after the Scenario
func cleanUpFetchedReleases(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	err := theRepositoryHasNoFetchedReleases(ctx)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}
