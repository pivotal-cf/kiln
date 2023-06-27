package scenario

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cucumber/godog"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/exp/slices"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

// resetTileRepository is to be run after the Scenario if the tile repo has been changed
func resetTileRepository(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	tileRepo, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}

	clean := exec.CommandContext(ctx, "git", "clean", "-ffd")
	clean.Dir = tileRepo
	_, err = runAndLogOnError(ctx, clean, false)
	if err != nil {
		return ctx, err
	}

	reset := exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD")
	reset.Dir = tileRepo
	_, err = runAndLogOnError(ctx, reset, false)
	if err != nil {
		return ctx, err
	}

	return ctx, nil
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

func iSetAVersionConstraintForRelease(ctx context.Context, versionConstraint, releaseName string) error {
	kfPath, err := kilnfilePath(ctx)
	if err != nil {
		return err
	}
	var spec cargo.Kilnfile
	err = loadFileAsYAML(kfPath, &spec)
	if err != nil {
		return err
	}
	specIndex := slices.IndexFunc(spec.Releases, func(release cargo.BOSHReleaseTarballSpecification) bool {
		return release.Name == releaseName
	})
	if specIndex == indexNotFound {
		return fmt.Errorf("index for component specification with name %q not found", releaseName)
	}
	spec.Releases[specIndex].Version = versionConstraint
	return saveAsYAML(kfPath, spec)
}

// iHaveARepositoryCheckedOutAtRevision checks out a repository at the filepath to a given revision
// Importantly, it also sets tilePath and tileVersion on kilnBakeScenario.
func iHaveARepositoryCheckedOutAtRevision(ctx context.Context, filePath, revision string) (context.Context, error) {
	repo, err := git.PlainOpen(filePath)
	if err != nil {
		return ctx, fmt.Errorf("opening the repository failed: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return ctx, fmt.Errorf("loading the worktree failed: %w", err)
	}

	revisionHash, err := repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return ctx, fmt.Errorf("resolving the given revision %q failed: %w", revision, err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  *revisionHash,
		Force: true,
	})
	if err != nil {
		return ctx, fmt.Errorf("checking out the revision %q at %q failed: %w", revision, revisionHash, err)
	}

	ctx = setTileVersion(ctx, strings.TrimPrefix(revision, "v"))

	return ctx, nil
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

	return nil
}

func iAddACompiledSReleaseSourceToTheKilnfile(ctx context.Context, bucketName string) error {
	keyID, accessKey, err := loadS3Credentials()
	if err != nil {
		return err
	}
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

	return saveAsYAML(kfPath, kf)
}

func iSetTheKilnfileStemcellVersionConstraint(ctx context.Context, versionConstraint string) error {
	spcePath, err := kilnfilePath(ctx)
	if err != nil {
		return err
	}
	var spec cargo.Kilnfile
	err = loadFileAsYAML(spcePath, &spec)
	if err != nil {
		return err
	}
	spec.Stemcell.Version = versionConstraint
	return saveAsYAML(spcePath, spec)
}

func theLockStemcellVersionIs(ctx context.Context, version string) error {
	lockPath, err := kilnfileLockPath(ctx)
	if err != nil {
		return err
	}
	var lock cargo.KilnfileLock
	err = loadFileAsYAML(lockPath, &lock)
	if err != nil {
		return err
	}
	if lock.Stemcell.Version != version {
		return fmt.Errorf("expected stemcell version to be %q but got %q", version, lock.Stemcell.Version)
	}
	return nil
}

func theKilnfileVersionForReleaseIs(ctx context.Context, releaseName, releaseVersion string) error {
	lockPath, err := kilnfilePath(ctx)
	if err != nil {
		return err
	}
	var kf cargo.Kilnfile
	err = loadFileAsYAML(lockPath, &kf)
	if err != nil {
		return err
	}
	releaseLock, err := kf.ComponentSpec(releaseName)
	if err != nil {
		return err
	}
	if releaseVersion != releaseLock.Version {
		return fmt.Errorf("the versions are not equal (%q != %q)", releaseVersion, releaseLock.Version)
	}
	return nil
}

func theKilnfileVersionForTheStemcellIs(ctx context.Context, stemcellVersion string) error {
	lockPath, err := kilnfilePath(ctx)
	if err != nil {
		return err
	}
	var kf cargo.Kilnfile
	err = loadFileAsYAML(lockPath, &kf)
	if err != nil {
		return err
	}
	if stemcellVersion != kf.Stemcell.Version {
		return fmt.Errorf("the versions are not equal (%q != %q)", stemcellVersion, kf.Stemcell.Version)
	}
	return nil
}
