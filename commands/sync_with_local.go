package commands

import (
	"fmt"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/yaml.v2"
	"log"
)

type SyncWithLocal struct {
	Options struct {
		Kilnfile        string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		ReleasesDir     string   `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
		ReleaseSourceID string   `long:"assume-release-source" description:"the release source to put in updated records" required:"true"`
		Variables       []string `short:"vr" long:"variable" description:"variable in key=value format"`
		VariablesFiles  []string `short:"vf" long:"variables-file" description:"path to variables file"`
	}
	fs                    billy.Filesystem
	kilnfileLoader        KilnfileLoader
	localReleaseDirectory LocalReleaseDirectory
	logger                *log.Logger
	remotePatherFactory   RemotePatherFactory
}

func NewSyncWithLocal(kilnfileLoader KilnfileLoader, fs billy.Filesystem, localReleaseDirectory LocalReleaseDirectory, remotePatherFactory RemotePatherFactory, logger *log.Logger) SyncWithLocal {
	return SyncWithLocal{
		fs:                    fs,
		kilnfileLoader:        kilnfileLoader,
		localReleaseDirectory: localReleaseDirectory,
		logger:                logger,
		remotePatherFactory:   remotePatherFactory,
	}
}

//go:generate counterfeiter -o ./fakes/remote_pather_factory.go --fake-name RemotePatherFactory . RemotePatherFactory
type RemotePatherFactory interface {
	RemotePather(sourceID string, kilnfile cargo.Kilnfile) (fetcher.RemotePather, error)
}

func (command SyncWithLocal) Execute(args []string) error {
	_, err := jhanda.Parse(&command.Options, args)
	if err != nil {
		return err
	}

	kilnfile, kilnfileLock, err := command.kilnfileLoader.LoadKilnfiles(
		command.fs,
		command.Options.Kilnfile,
		command.Options.VariablesFiles,
		command.Options.Variables,
	)
	if err != nil {
		return fmt.Errorf("couldn't load kilnfiles: %w", err) // untested
	}

	remotePather, err := command.remotePatherFactory.RemotePather(command.Options.ReleaseSourceID, kilnfile)
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
		remotePath, err := remotePather.RemotePath(release.Requirement{
			Name:            rel.Name,
			Version:         rel.Version,
			StemcellOS:      kilnfileLock.Stemcell.OS,
			StemcellVersion: kilnfileLock.Stemcell.Version,
		})
		if err != nil {
			return fmt.Errorf("couldn't generate a remote path for release %q: %w", rel.Name, err)
		}

		var matchingRelease *cargo.ReleaseLock
		for i := range kilnfileLock.Releases {
			if kilnfileLock.Releases[i].Name == rel.Name {
				matchingRelease = &kilnfileLock.Releases[i]
				break
			}
		}
		if matchingRelease == nil {
			return fmt.Errorf("the local release %q does not exist in the Kilnfile.lock", rel.Name)
		}

		matchingRelease.Version = rel.Version
		matchingRelease.SHA1 = rel.SHA1
		matchingRelease.RemoteSource = command.Options.ReleaseSourceID
		matchingRelease.RemotePath = remotePath

		command.logger.Printf("Updated %s to %s\n", rel.Name, rel.Version)
	}

	kilnfileLockFile, err := command.fs.Create(command.Options.Kilnfile + ".lock")
	if err != nil {
		return fmt.Errorf("couldn't open the Kilnfile.lock for updating: %w", err) // untested
	}

	defer kilnfileLockFile.Close()

	err = yaml.NewEncoder(kilnfileLockFile).Encode(kilnfileLock)
	if err != nil {
		return fmt.Errorf("couldn't write the updated Kilnfile.lock: %w", err) // untested
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