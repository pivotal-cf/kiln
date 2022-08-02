package scenario

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"golang.org/x/exp/slices"
	"os"
	"strings"

	"github.com/cucumber/godog"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/proofing"
	"github.com/pivotal-cf/kiln/pkg/tile"
)

const (
	indexNotFound = -1
)

// checkoutMainOnTileRepo is to be run after the Scenario if the tile repo has been changed
func checkoutMainOnTileRepo(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}
	return ctx, checkoutMain(repoPath)
}

// aTileIsCreated asserts the output tile exists
func aTileIsCreated(ctx context.Context) error {
	tilePath, err := defaultFilePathForTile(ctx)
	if err != nil {
		return err
	}
	_, err = os.Stat(tilePath)
	return err
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

// theTileContains checks that the filePaths exist in the tile
func theTileContains(ctx context.Context, _ string, table *godog.Table) error {
	tilePath, err := defaultFilePathForTile(ctx)
	if err != nil {
		return err
	}
	tile, err := zip.OpenReader(tilePath)
	if err != nil {
		return err
	}
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			_, err := tile.Open(cell.Value)
			if err != nil {
				return fmt.Errorf("tile did not contain file %s", cell.Value)
			}
		}
	}
	return success
}

func theTileOnlyContainsCompiledReleases(ctx context.Context) error {
	tilePath, err := defaultFilePathForTile(ctx)
	if err != nil {
		return err
	}
	metadataBuf, err := tile.ReadMetadataFromFile(tilePath)
	if err != nil {
		return err
	}

	var metadata struct {
		Releases []proofing.Release `yaml:"releases"`
	}
	err = yaml.Unmarshal(metadataBuf, &metadata)
	if err != nil {
		return err
	}

	for _, release := range metadata.Releases {
		helloReleaseTarball := bytes.NewBuffer(nil)
		_, err := tile.ReadReleaseFromFile(tilePath, release.Name, release.Version, helloReleaseTarball)
		if err != nil {
			return err
		}
		manifestBuf, err := component.ReadReleaseManifest(helloReleaseTarball)
		if err != nil {
			return err
		}
		err = validateAllPackagesAreCompiled(manifestBuf, release)
		if err != nil {
			return err
		}
	}

	return success
}

// validateAllPackagesAreCompiled asserts that if any package is listed under "packages", it is not complied.
// It does not ensure "compiled_packages" is populated.
func validateAllPackagesAreCompiled(manifestBuf []byte, release proofing.Release) error {
	var manifest struct {
		Packages []struct {
			Name string `yaml:"name"`
		} `yaml:"packages"`
	}
	err := yaml.Unmarshal(manifestBuf, &manifest)
	if err != nil {
		return err
	}
	if len(manifest.Packages) > 0 {
		sb := new(strings.Builder)
		sb.WriteString(fmt.Sprintf("release %s/%s contains un-compiled packages: ", release.Name, release.Version))
		for i, p := range manifest.Packages {
			sb.WriteString(p.Name)
			if i < len(manifest.Packages)-1 {
				sb.WriteString(", ")
			}
		}
		return errors.New(sb.String())
	}
	return success
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
