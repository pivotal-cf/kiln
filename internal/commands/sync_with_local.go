package commands

import (
	"fmt"
	"log"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/options"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type SyncWithLocal struct {
	Options struct {
		options.Standard
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

func (s SyncWithLocal) Execute(args []string) error {
	return Kiln{
		Wrapped: s,
		KilnfileStore: KilnfileStore{
			FS: s.fs,
		},
		StatFn: s.fs.Stat,
	}.Execute(args)
}

func (s SyncWithLocal) KilnExecute(args []string, parseOpts OptionsParseFunc) (cargo.KilnfileLock, bool, error) {
	kilnfile, kilnfileLock, _, err := parseOpts(args, &s.Options)
	if err != nil {
		return kilnfileLock, false, err
	}

	remotePather, err := s.remotePatherFinder(kilnfile, s.Options.ReleaseSourceID)
	if err != nil {
		return kilnfileLock, false, fmt.Errorf("couldn't load the release source: %w", err) // untested
	}

	s.logger.Printf("Finding releases in %s...\n", s.Options.ReleasesDir)
	releases, err := s.localReleaseDirectory.GetLocalReleases(s.Options.ReleasesDir)
	if err != nil {
		return kilnfileLock, false, fmt.Errorf("couldn't process releases in releases directory: %w", err) // untested
	}

	s.logger.Printf("Found %d releases on disk\n", len(releases))

	for _, rel := range releases {
		remotePath, err := remotePather.RemotePath(component.Spec{
			Name:            rel.Name,
			Version:         rel.Version,
			StemcellOS:      kilnfileLock.Stemcell.OS,
			StemcellVersion: kilnfileLock.Stemcell.Version,
		})
		if err != nil {
			return kilnfileLock, false, fmt.Errorf("couldn't generate a remote path for release %q: %w", rel.Name, err)
		}

		var matchingRelease *cargo.ComponentLock
		for i := range kilnfileLock.Releases {
			if kilnfileLock.Releases[i].Name == rel.Name {
				matchingRelease = &kilnfileLock.Releases[i]
				break
			}
		}
		if matchingRelease == nil {
			return kilnfileLock, false, fmt.Errorf("the local release %q does not exist in the Kilnfile.lock", rel.Name)
		}

		if s.Options.SkipSameVersion && matchingRelease.Version == rel.Version {
			s.logger.Printf("Skipping %s. Release version hasn't changed\n", rel.Name)
			continue
		}

		matchingRelease.Version = rel.Version
		matchingRelease.SHA1 = rel.SHA1
		matchingRelease.RemoteSource = s.Options.ReleaseSourceID
		matchingRelease.RemotePath = remotePath

		s.logger.Printf("Updated %s to %s\n", rel.Name, rel.Version)
	}

	return kilnfileLock, true, nil
}

func (s SyncWithLocal) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Update the Kilnfile.lock based on the local releases directory. Assume the given release source",
		ShortDescription: "update the Kilnfile.lock based on local releases",
		Flags:            s.Options,
	}
}
