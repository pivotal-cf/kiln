package commands

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/kiln/release"

	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/yaml.v2"
)

type UpdateReleaseCmd struct {
	Name                         string `short:"n" long:"name" required:"true" description:"name of release to update"`
	Version                      string `short:"v" long:"version" required:"true" description:"desired version of release"`
	ReleasesDir                  string `short:"r" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
	AllowOnlyPublishableReleases bool   `long:"allow-only-publishable-releases" description:"include releases that would not be shipped with the tile (development builds)"`
	panicCommand
}

func (u UpdateReleaseCmd) Runner(deps Dependencies) (CommandRunner, error) {
	return UpdateRelease{
		Name:                         u.Name,
		Version:                      u.Version,
		ReleasesDir:                  u.ReleasesDir,
		AllowOnlyPublishableReleases: u.AllowOnlyPublishableReleases,

		MultiReleaseSourceProvider: deps.ReleaseSourceRepo.MultiReleaseSource,
		Filesystem:                 deps.Filesystem,
		Logger:                     deps.OutLogger,
		KilnfileLock:               deps.KilnfileLock,
		KilnfileLockPath:           deps.KilnfileLockPath,
	}, nil
}

type UpdateRelease struct {
	Name                         string
	Version                      string
	ReleasesDir                  string
	AllowOnlyPublishableReleases bool

	MultiReleaseSourceProvider MultiReleaseSourceProvider
	Filesystem                 billy.Filesystem
	Logger                     *log.Logger
	KilnfileLock               cargo.KilnfileLock
	KilnfileLockPath           string
}

func (u UpdateRelease) Run(_ []string) error {
	releaseSource := u.MultiReleaseSourceProvider(u.AllowOnlyPublishableReleases)

	u.Logger.Println("Searching for the release...")
	remoteRelease, found, err := releaseSource.GetMatchedRelease(release.Requirement{
		Name:            u.Name,
		Version:         u.Version,
		StemcellOS:      u.KilnfileLock.Stemcell.OS,
		StemcellVersion: u.KilnfileLock.Stemcell.Version,
	})
	if err != nil {
		return fmt.Errorf("error finding the release: %w", err)
	}
	if !found {
		return fmt.Errorf("couldn't find %q %s in any release source", u.Name, u.Version)
	}

	localRelease, err := releaseSource.DownloadRelease(u.ReleasesDir, remoteRelease, 0)
	if err != nil {
		return fmt.Errorf("error downloading the release: %w", err)
	}

	var matchingRelease *cargo.ReleaseLock
	for i := range u.KilnfileLock.Releases {
		if u.KilnfileLock.Releases[i].Name == u.Name {
			matchingRelease = &u.KilnfileLock.Releases[i]
			break
		}
	}
	if matchingRelease == nil {
		return fmt.Errorf("no release named %q exists in your Kilnfile.lock", u.Name)
	}

	matchingRelease.Version = localRelease.Version
	matchingRelease.SHA1 = localRelease.SHA1
	matchingRelease.RemoteSource = remoteRelease.SourceID
	matchingRelease.RemotePath = remoteRelease.RemotePath

	updatedLockFileYAML, err := yaml.Marshal(u.KilnfileLock)
	if err != nil {
		return fmt.Errorf("error marshaling the Kilnfile.lock: %w", err) // untestable
	}

	lockFile, err := u.Filesystem.Create(u.KilnfileLockPath) // overwrites the file
	if err != nil {
		return fmt.Errorf("error reopening the Kilnfile.lock for writing: %w", err)
	}

	_, err = lockFile.Write(updatedLockFileYAML)
	if err != nil {
		return fmt.Errorf("error writing to Kilnfile.lock: %w", err)
	}

	u.Logger.Printf("Updated %s to %s. DON'T FORGET TO MAKE A COMMIT AND PR\n", u.Name, u.Version)
	return nil
}
