package scenario

import (
	"context"
	"errors"
	"fmt"
	"github.com/cucumber/godog"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

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
	releaseLock, err := lock.FindBOSHReleaseWithName(releaseName)
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
func iHaveATileDirectory(ctx context.Context, tileDirectory string) (context.Context, error) {
	if err := os.RemoveAll(filepath.Join(tileDirectory, ".git")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ctx, fmt.Errorf("failed to remove .git directory: %w", err)
	}
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return ctx, fmt.Errorf("failed to create new copy of tile directory: %w", err)
	}

	dir, err := copyTileDirectory(tmpDir, tileDirectory)
	if err != nil {
		return ctx, err
	}

	version, err := os.ReadFile(filepath.Join(dir, "version"))
	if err != nil {
		version = []byte("0.1.0")
	}

	ctx = setTileVersion(ctx, strings.TrimSpace(string(version)))
	ctx = setTileRepoPath(ctx, dir)

	return ctx, nil
}

func copyTileDirectory(dir, tileDirectory string) (string, error) {
	testTileDir := filepath.Join(dir, filepath.Base(tileDirectory))
	if err := filepath.Walk(tileDirectory, copyDir(testTileDir, tileDirectory)); err != nil {
		return "", fmt.Errorf("failed to copy tile directory: %w", err)
	}
	if err := executeAndWrapError(testTileDir, "git", "init"); err != nil {
		return "", fmt.Errorf("tile path is not a repository: initalizing failed: %w", err)
	}
	if err := executeAndWrapError(testTileDir, "git", "config", "user.email", "test-git-user@example.com"); err != nil {
		return "", err
	}
	if err := executeAndWrapError(testTileDir, "git", "config", "user.name", "test-git-user"); err != nil {
		return "", err
	}
	if err := executeAndWrapError(testTileDir, "git", "add", "."); err != nil {
		return "", fmt.Errorf("tile path is not a repository: adding initial files failed: %w", err)
	}
	if err := executeAndWrapError(testTileDir, "git", "commit", "-m", "initial commit"); err != nil {
		return "", fmt.Errorf("tile path is not a repository: adding initial files failed: %w", err)
	}
	return testTileDir, nil
}

func copyDir(dstDir, srcDir string) filepath.WalkFunc {
	return func(srcPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rp, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return err
		}
		createPath := filepath.Join(dstDir, rp)
		if info.IsDir() {
			_, err := os.Stat(createPath)
			if err != nil {
				return os.Mkdir(createPath, info.Mode().Perm())
			}
			return nil
		}
		dst, err := os.Create(createPath)
		if err != nil {
			return err
		}
		defer closeAndIgnoreErr(dst)
		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer closeAndIgnoreErr(src)
		_, err = io.Copy(dst, src)
		return err
	}
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
	releaseLock, err := kf.BOSHReleaseTarballSpecification(releaseName)
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
