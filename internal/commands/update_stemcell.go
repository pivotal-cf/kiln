package commands

import (
	"fmt"
	"log"
	"strings"

	"github.com/pivotal-cf/kiln/pkg/cargo"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

type UpdateStemcell struct {
	Options struct {
		flags.Standard

		Version     string `short:"v"  long:"version"            required:"true"    description:"desired version of stemcell"`
		ReleasesDir string `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
	}
	FS                         billy.Filesystem
	MultiReleaseSourceProvider MultiReleaseSourceProvider
	Logger                     *log.Logger
}

func (update UpdateStemcell) Execute(args []string) error {
	_, err := flags.LoadFlagsWithDefaults(&update.Options, args, update.FS.Stat)
	if err != nil {
		return err
	}

	kilnfile, kilnfileLock, err := update.Options.Standard.LoadKilnfiles(update.FS, nil)
	if err != nil {
		return fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	var releaseVersionConstraint *semver.Constraints

	trimmedInputVersion := strings.TrimSpace(update.Options.Version)

	latestStemcellVersion, err := semver.NewVersion(trimmedInputVersion)
	if err != nil {
		return fmt.Errorf("invalid stemcell version (please enter a valid version): %w", err)
	}

	kilnStemcellVersion := kilnfile.Stemcell.Version
	releaseVersionConstraint, err = semver.NewConstraint(kilnStemcellVersion)

	if err != nil {
		return fmt.Errorf("invalid stemcell constraint in kilnfile: %w", err)
	}

	if !releaseVersionConstraint.Check(latestStemcellVersion) {
		update.Logger.Println("Latest version does not satisfy the stemcell version constraint in kilnfile. Nothing to update.")
		return nil
	}

	currentStemcellVersion, _ := semver.NewVersion(kilnfileLock.Stemcell.Version)

	if currentStemcellVersion.Equal(latestStemcellVersion) {
		update.Logger.Println("Stemcell is up-to-date. Nothing to update for product")
		return nil
	}

	releaseSources := update.MultiReleaseSourceProvider(kilnfile, false)

	for i, existingLock := range kilnfileLock.Releases {
		update.Logger.Printf("Updating release %q with stemcell %s %s...", existingLock.Name, kilnfileLock.Stemcell.OS, trimmedInputVersion)

		spec, found := kilnfile.ComponentSpec(existingLock.Name)
		if !found {
			return cargo.ErrorSpecNotFound(existingLock.Name)
		}
		spec.StemcellOS = kilnfileLock.Stemcell.OS
		spec.StemcellVersion = trimmedInputVersion
		spec.Version = existingLock.Version

		releaseCache, err := releaseSources.GetReleaseCache()
		if err != nil {
			return err
		}
		remote, err := releaseCache.GetMatchedRelease(spec)
		if err != nil {
			releaseSource, err := releaseSources.FindByID(spec.ReleaseSource)
			if err != nil {
				return err
			}
			remote, err = releaseSource.GetMatchedRelease(spec)
			if err != nil {
				return fmt.Errorf("failed to get release %q: %w", spec.Name, err)
			}
		}
		if remote.RemotePath == existingLock.RemotePath && remote.RemoteSource == existingLock.RemoteSource {
			update.Logger.Printf("No change for release %q\n", existingLock.Name)
			continue
		}
		downloadSource, err := releaseSources.FindByID(remote.RemoteSource)
		if err != nil {
			return err
		}
		local, err := downloadSource.DownloadRelease(update.Options.ReleasesDir, remote)
		if err != nil {
			return fmt.Errorf("while downloading release %q, encountered error: %w", existingLock.Name, err)
		}
		kilnfileLock.Releases[i].SHA1 = local.SHA1
		kilnfileLock.Releases[i].RemotePath = remote.RemotePath
		kilnfileLock.Releases[i].RemoteSource = remote.RemoteSource
	}

	kilnfileLock.Stemcell.Version = trimmedInputVersion

	err = update.Options.Standard.SaveKilnfileLock(update.FS, kilnfileLock)
	if err != nil {
		return err
	}

	update.Logger.Println("Finished updating Kilnfile.lock")
	return nil
}

func (update UpdateStemcell) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Updates stemcell and release information in Kilnfile.lock",
		ShortDescription: "updates stemcell and release information in Kilnfile.lock",
		Flags:            update.Options,
	}
}
