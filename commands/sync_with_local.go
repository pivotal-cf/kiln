package commands

import (
	"fmt"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/yaml.v2"
	"log"
)

type SyncWithLocalCmd struct {
	ReleasesDir     string   `short:"r" long:"releases-directory"    default:"releases" description:"path to a directory to download releases into"`
	ReleaseSourceID string   `          long:"assume-release-source" required:"true"    description:"the release source to put in updated records"`
	panicCommand
}

func (s SyncWithLocalCmd) Runner(deps Dependencies) (CommandRunner, error) {
	return SyncWithLocal{
		ReleasesDir: s.ReleasesDir,
		ReleaseSourceID: s.ReleaseSourceID,

		FS: deps.Filesystem,
		KilnfileLock: deps.KilnfileLock,
		KilnfileLockPath: deps.KilnfileLockPath,
		LocalReleaseDirectory: deps.LocalReleaseDirectory,
		Logger: deps.OutLogger,
		RemotePatherFinder: deps.ReleaseSourceRepo.FindRemotePather,
	}, nil
}

//go:generate counterfeiter -o ./fakes/remote_pather_finder.go --fake-name RemotePatherFinder . RemotePatherFinder
type RemotePatherFinder func(string) (fetcher.RemotePather, error)

type SyncWithLocal struct {
	ReleasesDir           string
	ReleaseSourceID       string

	FS                    billy.Filesystem
	KilnfileLock cargo.KilnfileLock
	KilnfileLockPath string
	LocalReleaseDirectory LocalReleaseDirectory
	Logger                *log.Logger
	RemotePatherFinder    RemotePatherFinder
}

func (command SyncWithLocal) Run(args []string) error {
	remotePather, err := command.RemotePatherFinder(command.ReleaseSourceID)
	if err != nil {
		return fmt.Errorf("couldn't load the release source: %w", err) // untested
	}

	command.Logger.Printf("Finding releases in %s...\n", command.ReleasesDir)
	releases, err := command.LocalReleaseDirectory.GetLocalReleases(command.ReleasesDir)
	if err != nil {
		return fmt.Errorf("couldn't process releases in releases directory: %w", err) // untested
	}

	command.Logger.Printf("Found %d releases on disk\n", len(releases))

	for _, rel := range releases {
		remotePath, err := remotePather.RemotePath(release.Requirement{
			Name:            rel.Name,
			Version:         rel.Version,
			StemcellOS:      command.KilnfileLock.Stemcell.OS,
			StemcellVersion: command.KilnfileLock.Stemcell.Version,
		})
		if err != nil {
			return fmt.Errorf("couldn't generate a remote path for release %q: %w", rel.Name, err)
		}

		var matchingRelease *cargo.ReleaseLock
		for i := range command.KilnfileLock.Releases {
			if command.KilnfileLock.Releases[i].Name == rel.Name {
				matchingRelease = &command.KilnfileLock.Releases[i]
				break
			}
		}
		if matchingRelease == nil {
			return fmt.Errorf("the local release %q does not exist in the Kilnfile.lock", rel.Name)
		}

		matchingRelease.Version = rel.Version
		matchingRelease.SHA1 = rel.SHA1
		matchingRelease.RemoteSource = command.ReleaseSourceID
		matchingRelease.RemotePath = remotePath

		command.Logger.Printf("Updated %s to %s\n", rel.Name, rel.Version)
	}

	kilnfileLockFile, err := command.FS.Create(command.KilnfileLockPath)
	if err != nil {
		return fmt.Errorf("couldn't open the Kilnfile.lock for updating: %w", err) // untested
	}

	defer kilnfileLockFile.Close()

	err = yaml.NewEncoder(kilnfileLockFile).Encode(command.KilnfileLock)
	if err != nil {
		return fmt.Errorf("couldn't write the updated Kilnfile.lock: %w", err) // untested
	}

	return nil
}
