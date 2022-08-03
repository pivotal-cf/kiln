package scenario

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cucumber/godog"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/exp/slices"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

// checkoutMainOnTileRepo is to be run after the Scenario if the tile repo has been changed
func checkoutMainOnTileRepo(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	return ctx, checkoutMain(repoPath)
}

func theLockSpecifiesVersionForRelease(ctx context.Context, releaseVersion, releaseName string) error {
	lockPath, err := kilnfileLockPath(ctx)
	if err != nil {
		return err
	}
	var lock cargo.KilnfileLock
	err = loadFileAsYAML(lockPath, &lock)
	if err != nil {
		return err
	}
	releaseLock, err := lock.FindReleaseWithName(releaseName)
	if err != nil {
		return err
	}
	if releaseLock.Version != releaseVersion {
		return fmt.Errorf("expected %q to equal %q", releaseLock.Version, releaseVersion)
	}
	return nil
}

// cleanUpFetchedReleases should be run after the Scenario
func cleanUpFetchedReleases(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	err := theRepositoryHasNoFetchedReleases(ctx)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}

func iSetAVersionConstraintForRelease(ctx context.Context, versionConstraint, releaseName string) error {
	spcePath, err := kilnfileLockPath(ctx)
	if err != nil {
		return err
	}
	var spec cargo.Kilnfile
	err = loadFileAsYAML(spcePath, &spec)
	specIndex := slices.IndexFunc(spec.Releases, func(release cargo.ComponentSpec) bool {
		return release.Name == releaseName
	})
	if specIndex == indexNotFound {
		return cargo.ErrorSpecNotFound(releaseName)
	}
	spec.Releases[specIndex].Version = versionConstraint
	return saveAsYAML(spcePath, spec)
}

// iHaveARepositoryCheckedOutAtRevision checks out a repository at the filepath to a given revision
// Importantly, it also sets tilePath and tileVersion on kilnBakeScenario.
func iHaveARepositoryCheckedOutAtRevision(ctx context.Context, filePath, revision string) (context.Context, error) {
	repo, err := git.PlainOpen(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening the repository failed: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("loading the worktree failed: %w", err)
	}

	revisionHash, err := repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, fmt.Errorf("resolving the given revision %q failed: %w", revision, err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  *revisionHash,
		Force: true,
	})
	if err != nil {
		return nil, fmt.Errorf("checking out the revision %q at %q failed: %w", revision, revisionHash, err)
	}

	ctx = setTileVersion(ctx, strings.TrimPrefix(revision, "v"))

	return ctx, success
}

// theRepositoryHasNoFetchedReleases deletes fetched releases, if any.
func theRepositoryHasNoFetchedReleases(ctx context.Context) error {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	releaseDirectoryName := repoPath + "/releases"
	releaseDirectory, err := os.Open(releaseDirectoryName)
	if err != nil {
		return fmt.Errorf("unable to open release directory [ %s ]: %w", releaseDirectoryName, err)
	}

	defer closeAndIgnoreErr(releaseDirectory)

	releaseFiles, err := releaseDirectory.Readdir(0)
	if err != nil {
		return fmt.Errorf("unable to read files from [ %s ]: %w", releaseDirectory.Name(), err)
	}

	for f := range releaseFiles {
		file := releaseFiles[f]

		fileName := file.Name()
		filePath := releaseDirectory.Name() + "/" + fileName

		// Preserve dot files, namely `.gitignore`
		if strings.HasPrefix(fileName, ".") {
			continue
		}

		err = os.Remove(filePath)
		if err != nil {
			return fmt.Errorf("unable to remove file [ %s ]: %w", filePath, err)
		}
	}

	return success
}

func iAddACompiledSReleaseSourceToTheKilnfile(ctx context.Context, bucketName string) error {
	keyID, accessKey, err := loadS3Credentials()
	kfPath, err := kilnfilePath(ctx)
	if err != nil {
		return err
	}

	var kf cargo.Kilnfile
	err = loadFileAsYAML(kfPath, &kf)
	if err != nil {
		return err
	}

	for _, rs := range kf.ReleaseSources {
		if rs.Bucket == bucketName {
			return nil
		}
	}

	kf.ReleaseSources = append(kf.ReleaseSources, cargo.ReleaseSourceConfig{
		Type:            component.ReleaseSourceTypeS3,
		Bucket:          bucketName,
		PathTemplate:    "{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz",
		Region:          "us-west-1",
		Publishable:     true,
		AccessKeyId:     keyID,
		SecretAccessKey: accessKey,
	})

	setPublishableReleaseSource(ctx, bucketName)

	return saveAsYAML(kfPath, kf)
}
