package commands

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/kiln/internal/commands/flags"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type SyncWithLocal struct {
	Options struct {
		flags.Standard

		ReleasesDir     string `short:"rd" long:"releases-directory"  default:"releases" description:"path to a directory to download releases into"`
		ReleaseSourceID string `           long:"assume-release-source"  required:"true" description:"the release source to put in updated records"`
		SkipSameVersion bool   `           long:"skip-same-version"                      description:"only update the Kilnfile.lock when the release version has changed'"`
	}
	fs                    billy.Filesystem
	localReleaseDirectory LocalReleaseDirectory
	logger                *log.Logger
	remotePatherFinder    RemotePatherFinder
}

func NewSyncWithLocal(fs billy.Filesystem, localReleaseDirectory LocalReleaseDirectory, remotePatherFinder RemotePatherFinder, logger *log.Logger) SyncWithLocal {
	return SyncWithLocal{
		fs:                    fs,
		localReleaseDirectory: localReleaseDirectory,
		logger:                logger,
		remotePatherFinder:    remotePatherFinder,
	}
}

//counterfeiter:generate -o ./fakes/remote_pather_finder.go --fake-name RemotePatherFinder . RemotePatherFinder
type RemotePatherFinder func(cargo.Kilnfile, string) (component.RemotePather, error)

func (command SyncWithLocal) Execute(args []string) error {
	_, err := flags.LoadFlagsWithDefaults(&command.Options, args, command.fs.Stat)
	if err != nil {
		return err
	}

	kilnfile, kilnfileLock, err := command.Options.Standard.LoadKilnfiles(command.fs, nil)
	if err != nil {
		return fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	remotePather, err := command.remotePatherFinder(kilnfile, command.Options.ReleaseSourceID)
	if err != nil {
		return fmt.Errorf("couldn't load the release source: %w", err) // untested
	}

	command.logger.Printf("Finding releases in %s...\n", command.Options.ReleasesDir)
	releases, err := command.localReleaseDirectory.GetLocalReleases(command.Options.ReleasesDir)
	if err != nil {
		return fmt.Errorf("couldn't process releases in releases directory: %w", err) // untested
	}

	command.logger.Printf("Found %d releases on disk\n", len(releases))

	for _, rel := range releases {
		remotePath, err := remotePather.RemotePath(cargo.BOSHReleaseSpecification{
			Name:            rel.Lock.Name,
			Version:         rel.Lock.Version,
			StemcellOS:      kilnfileLock.Stemcell.OS,
			StemcellVersion: kilnfileLock.Stemcell.Version,
		})
		if err != nil {
			return fmt.Errorf("couldn't generate a remote path for release %q: %w", rel.Lock.Name, err)
		}

		var matchingRelease *cargo.BOSHReleaseLock
		for i := range kilnfileLock.Releases {
			if kilnfileLock.Releases[i].Name == rel.Lock.Name {
				matchingRelease = &kilnfileLock.Releases[i]
				break
			}
		}
		if matchingRelease == nil {
			return fmt.Errorf("the local release %q does not exist in the Kilnfile.lock", rel.Lock.Name)
		}

		if command.Options.SkipSameVersion && matchingRelease.Version == rel.Lock.Version {
			command.logger.Printf("Skipping %s. Release version hasn't changed\n", rel.Lock.Name)
			continue
		}

		matchingRelease.Version = rel.Lock.Version
		matchingRelease.SHA1 = rel.Lock.SHA1
		matchingRelease.RemoteSource = command.Options.ReleaseSourceID
		matchingRelease.RemotePath = remotePath

		command.logger.Printf("Updated %s to %s\n", rel.Lock.Name, rel.Lock.Version)
	}

	err = command.Options.SaveKilnfileLock(command.fs, kilnfileLock)
	if err != nil {
		return err
	}

	return nil
}

func (command SyncWithLocal) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Update the Kilnfile.lock based on the local releases directory. Assume the given release source",
		ShortDescription: "update the Kilnfile.lock based on local releases",
		Flags:            command.Options,
	}
}
