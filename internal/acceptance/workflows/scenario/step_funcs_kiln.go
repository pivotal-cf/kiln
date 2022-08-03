package scenario

import (
	"context"
)

const successfullyFlagValue = "try to "

type requireSuccessFlag string

func (f requireSuccessFlag) isSet() bool {
	return f == ""
}

// iInvokeKilnBake invokes kiln bake with tileVersion provided by iHaveARepositoryCheckedOutAtRevision
func iInvokeKilnBake(ctx context.Context, requireSuccess string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	version, err := tileVersion(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "bake", "--version", version)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, requireSuccessFlag(requireSuccess).isSet())
}

func iInvokeKilnCacheCompiledReleases(ctx context.Context, requireSuccess requireSuccessFlag) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return ctx, err
	}
	uploadTargetID, err := publishableReleaseSource(ctx)
	if err != nil {
		return ctx, err
	}
	env, err := environment(ctx)
	if err != nil {
		return ctx, err
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
	return runAndLogOnError(ctx, cmd, requireSuccess.isSet())
}

// iInvokeKilnFetch fetches releases. It provides the command with the GitHub token (used for hello-release).
func iInvokeKilnFetch(ctx context.Context, requireSuccess string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "fetch", "--no-confirm", "--variable", "github_token="+token)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, requireSuccessFlag(requireSuccess).isSet())
}

func iInvokeKilnFindReleaseVersion(ctx context.Context, requireSuccess, releaseName string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "find-release-version", "--release", releaseName,
		"--variable", "github_token="+token)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, requireSuccessFlag(requireSuccess).isSet())
}

func iInvokeKilnHelp(ctx context.Context, requireSuccess string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "help")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, requireSuccessFlag(requireSuccess).isSet())
}

func iInvokeKilnUpdateRelease(ctx context.Context, requireSuccess, releaseName, releaseVersion string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "update-release", "--name", releaseName, "--version", releaseVersion,
		"--without-download",
		"--variable", "github_token="+token,
	)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, requireSuccessFlag(requireSuccess).isSet())
}

func iInvokeKilnVersion(ctx context.Context, requireSuccess string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "version")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, requireSuccessFlag(requireSuccess).isSet())
}

func kilnValidateSucceeds(ctx context.Context) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "validate", "--variable", "github_token=banana")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, true)
}

func iInvokeKilnBooBoo(ctx context.Context, requireSuccess string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "boo-boo")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, requireSuccessFlag(requireSuccess).isSet())
}

func iInvokeKilnCommandWithFlagBooBoo(ctx context.Context, requireSuccess, command string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, command, "--boo-boo")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, requireSuccessFlag(requireSuccess).isSet())
}
