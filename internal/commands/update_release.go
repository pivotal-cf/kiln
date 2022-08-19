package commands

import (
	"fmt"
	"log"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type UpdateRelease struct {
	Options struct {
		flags.Standard

		Name                         string `short:"n"  long:"name"                            required:"true"                         description:"name of release to update"`
		Version                      string `short:"v"  long:"version"                         required:"true"                         description:"desired version of release"`
		ReleasesDir                  string `short:"rd" long:"releases-directory"                              default-path:"releases" description:"path to a directory to download releases into"`
		AllowOnlyPublishableReleases bool   `           long:"allow-only-publishable-releases"                                         description:"include releases that would not be shipped with the tile (development builds)"`
		WithoutDownload              bool   `           long:"without-download"                                                        description:"updates releases without downloading them"`
	}
	multiReleaseSourceProvider MultiReleaseSourceProvider
	filesystem                 billy.Filesystem
	logger                     *log.Logger
}

func NewUpdateRelease(logger *log.Logger, filesystem billy.Filesystem, multiReleaseSourceProvider MultiReleaseSourceProvider) UpdateRelease {
	return UpdateRelease{
		logger:                     logger,
		multiReleaseSourceProvider: multiReleaseSourceProvider,
		filesystem:                 filesystem,
	}
}

func (u UpdateRelease) Execute(args []string) error {
	_, err := flags.LoadFlagsWithDefaults(&u.Options, args, u.filesystem.Stat)
	if err != nil {
		return err
	}

	kilnfile, kilnfileLock, err := u.Options.Standard.LoadKilnfiles(u.filesystem, nil)
	if err != nil {
		return fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	releaseLock, err := kilnfileLock.FindReleaseWithName(u.Options.Name)
	if err != nil {
		return fmt.Errorf(
			"no release named %q exists in your Kilnfile.lock - try removing the -release, -boshrelease, or -bosh-release suffix if present",
			u.Options.Name,
		)
	}
	releaseSpec, ok := kilnfile.ComponentSpec(u.Options.Name)
	if !ok {
		return cargo.ErrorSpecNotFound(u.Options.Name)
	}

	releaseVersionConstraint := releaseSpec.Version
	if u.Options.Version != "" {
		releaseVersionConstraint = u.Options.Version
	}

	releaseSource := u.multiReleaseSourceProvider(kilnfile, u.Options.AllowOnlyPublishableReleases)

	u.logger.Println("Searching for the release...")

	var localRelease component.Local
	var remoteRelease component.Lock
	var newVersion, newSHA1, newSourceID, newRemotePath string
	if u.Options.WithoutDownload {
		remoteRelease, err = releaseSource.FindReleaseVersion(component.Spec{
			Name:             u.Options.Name,
			Version:          releaseVersionConstraint,
			StemcellVersion:  kilnfileLock.Stemcell.Version,
			StemcellOS:       kilnfileLock.Stemcell.OS,
			GitHubRepository: releaseSpec.GitHubRepository,
		}, false)

		if err != nil {
			if component.IsErrNotFound(err) {
				return fmt.Errorf("error finding the release: %w", err)
			}
			return fmt.Errorf("couldn't find %q %s in any release source", u.Options.Name, u.Options.Version)
		}

		newVersion = remoteRelease.Version
		newSHA1 = remoteRelease.SHA1
		newSourceID = remoteRelease.RemoteSource
		newRemotePath = remoteRelease.RemotePath

	} else {
		remoteRelease, err = releaseSource.GetMatchedRelease(component.Spec{
			Name:             u.Options.Name,
			Version:          u.Options.Version,
			StemcellOS:       kilnfileLock.Stemcell.OS,
			StemcellVersion:  kilnfileLock.Stemcell.Version,
			GitHubRepository: releaseSpec.GitHubRepository,
		})

		if err != nil {
			if component.IsErrNotFound(err) {
				return fmt.Errorf("error finding the release: %w", err)
			}
			return fmt.Errorf("couldn't find %q %s in any release source", u.Options.Name, u.Options.Version)
		}

		localRelease, err = releaseSource.DownloadRelease(u.Options.ReleasesDir, remoteRelease)
		if err != nil {
			return fmt.Errorf("error downloading the release: %w", err)
		}
		newVersion = localRelease.Version
		newSHA1 = localRelease.SHA1
		newSourceID = remoteRelease.RemoteSource
		newRemotePath = remoteRelease.RemotePath
	}

	if releaseLock.Version == newVersion && releaseLock.SHA1 == newSHA1 && releaseLock.RemoteSource == newSourceID && releaseLock.RemotePath == newRemotePath {
		u.logger.Println("Neither the version nor remote location of the release changed. No changes made.")
		return nil
	}

	releaseLock.Version = newVersion
	releaseLock.SHA1 = newSHA1
	releaseLock.RemoteSource = newSourceID
	releaseLock.RemotePath = newRemotePath

	_ = kilnfileLock.UpdateReleaseLockWithName(u.Options.Name, releaseLock)

	err = u.Options.Standard.SaveKilnfileLock(u.filesystem, kilnfileLock)
	if err != nil {
		return err
	}

	u.logger.Printf("Updated %s to %s. DON'T FORGET TO MAKE A COMMIT AND PR\n", u.Options.Name, u.Options.Version)
	return nil
}

func (u UpdateRelease) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Bumps a release to a new version in Kilnfile.lock",
		ShortDescription: "bumps a release to a new version",
		Flags:            u.Options,
	}
}
