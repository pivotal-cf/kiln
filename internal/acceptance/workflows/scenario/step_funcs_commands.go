package scenario

import (
	"context"
)

const successfullyFlagValue = "try to "

type tryFlag string

func (f tryFlag) isSet() bool {
	return f == successfullyFlagValue
}

func (f tryFlag) requireSuccess() bool {
	return !f.isSet()
}

// iInvokeKilnBake invokes kiln bake with tileVersion provided by iHaveARepositoryCheckedOutAtRevision
func iInvokeKilnBake(ctx context.Context, try string) (context.Context, error) {
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
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnCacheCompiledReleases(ctx context.Context, try tryFlag) (context.Context, error) {
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
	return runAndLogOnError(ctx, cmd, try.isSet())
}

// iInvokeKilnFetch fetches releases. It provides the command with the GitHub token (used for hello-release).
func iInvokeKilnFetch(ctx context.Context, try string) (context.Context, error) {
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
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnFindReleaseVersion(ctx context.Context, try, releaseName string) (context.Context, error) {
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
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnHelp(ctx context.Context, try string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "help")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnUpdateRelease(ctx context.Context, try, releaseName, releaseVersion string) (context.Context, error) {
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
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnVersion(ctx context.Context, try string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "version")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
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

func iInvokeKilnBooBoo(ctx context.Context, try string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "boo-boo")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnCommandWithFlagBooBoo(ctx context.Context, try, command string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, command, "--boo-boo")
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnReleaseNotes(ctx context.Context, try, initialRevision, finalRevision string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "release-notes",
		"--release-date", "2022-07-27",
		"--github-issue-milestone", "Release-2022-001",
		"--update-docs", "../scenario/fixtures/release_notes.md.erb",
		"--kilnfile", "Kilnfile",
		initialRevision, finalRevision,
	)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnFindStemcellVersion(ctx context.Context, try string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "find-stemcell-version",
		"--variable", "github_token=banana",
	)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}

func iInvokeKilnUpdateStemcellWithVersion(ctx context.Context, try, version string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	token, err := githubToken(ctx)
	if err != nil {
		return ctx, err
	}
	cmd := kilnCommand(ctx, "update-stemcell",
		"--version", version,
		"--variable", "github_token="+token,
	)
	cmd.Dir = repoPath
	return runAndLogOnError(ctx, cmd, tryFlag(try).requireSuccess())
}
