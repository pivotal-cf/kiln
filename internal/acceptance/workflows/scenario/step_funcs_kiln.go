package scenario

import "context"

// iInvokeKilnBake invokes kiln bake with tileVersion provided by iHaveARepositoryCheckedOutAtRevision
func iInvokeKilnBake(ctx context.Context) error {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	version, err := tileVersion(ctx)
	if err != nil {
		return err
	}
	cmd := kilnCommand(ctx, "bake", "--version", version)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd)
}

func iInvokeKilnCacheCompiledReleases(ctx context.Context) error {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return err
	}
	uploadTargetID, err := publishableReleaseSource(ctx)
	if err != nil {
		return err
	}
	env, err := environment(ctx)
	if err != nil {
		return err
	}

	cmd := kilnCommand(ctx, "cache-compiled-releases",
		"--variable", "github_token="+token,
		"--upload-target-id", uploadTargetID,
		"--name", "hello",
		"--om-username", env.OpsManager.Username,
		"--om-password", env.OpsManager.Password,
		"--om-target", env.OpsManager.URL,
		"--om-private-key", env.OpsManagerPrivateKey,
	)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd)
}

// iInvokeKilnFetch fetches releases. It provides the command with the GitHub token (used for hello-release).
func iInvokeKilnFetch(ctx context.Context) error {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return err
	}
	cmd := kilnCommand(ctx, "fetch", "--no-confirm", "--variable", "github_token="+token)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd)
}

func iInvokeKilnFindReleaseVersion(ctx context.Context, releaseName string) error {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return err
	}
	cmd := kilnCommand(ctx, "find-release-version", "--release", releaseName,
		"--variable", "github_token="+token)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd)
}

func iInvokeKilnUpdateRelease(ctx context.Context, releaseName, releaseVersion string) error {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return err
	}
	cmd := kilnCommand(ctx, "update-release", "--name", releaseName, "--version", releaseVersion,
		"--without-download",
		"--variable", "github_token="+token,
	)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd)
}

func iInvokeKilnVersion(ctx context.Context) error {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	cmd := kilnCommand(ctx, "version")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd)
}

func kilnValidateSucceeds(ctx context.Context) error {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	cmd := kilnCommand(ctx, "validate", "--variable", "github_token=banana")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd)
}
