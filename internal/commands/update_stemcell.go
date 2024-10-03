package commands

import (
	"fmt"
	"log"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
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
	_, err := flags.LoadWithDefaultFilePaths(&update.Options, args, update.FS.Stat)
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

	releaseSource := update.MultiReleaseSourceProvider(kilnfile, false)

	for i, rel := range kilnfileLock.Releases {
		update.Logger.Printf("Updating release %q with stemcell %s %s...", rel.Name, kilnfileLock.Stemcell.OS, trimmedInputVersion)

		spec, err := kilnfile.BOSHReleaseTarballSpecification(rel.Name)
		if err != nil {
			return err
		}
		spec.StemcellOS = kilnfileLock.Stemcell.OS
		spec.StemcellVersion = trimmedInputVersion
		spec.Version = rel.Version

		remote, err := releaseSource.GetMatchedRelease(spec)
		if err != nil {
			return fmt.Errorf("while finding release %q, encountered error: %w", rel.Name, err)
		}
		if component.IsErrNotFound(err) {
			return fmt.Errorf("couldn't find release %q", rel.Name)
		}

		if remote.RemotePath == rel.RemotePath && remote.RemoteSource == rel.RemoteSource {
			update.Logger.Printf("No change for release %q\n", rel.Name)
			continue
		}

		local, err := releaseSource.DownloadRelease(update.Options.ReleasesDir, remote)
		if err != nil {
			return fmt.Errorf("while downloading release %q, encountered error: %w", rel.Name, err)
		}

		lock := &kilnfileLock.Releases[i]
		lock.SHA1 = local.Lock.SHA1
		lock.RemotePath = remote.RemotePath
		lock.RemoteSource = remote.RemoteSource
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
