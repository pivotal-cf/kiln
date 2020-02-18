package commands

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/kiln/fetcher"

	"github.com/pivotal-cf/kiln/release"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/src-d/go-billy.v4"
)

type UpdateRelease struct {
	Options struct {
		Kilnfile                     string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		Name                         string   `short:"n" long:"name" required:"true" description:"name of release to update"`
		Version                      string   `short:"v" long:"version" required:"true" description:"desired version of release"`
		ReleasesDir                  string   `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
		Variables                    []string `short:"vr" long:"variable" description:"variable in key=value format"`
		VariablesFiles               []string `short:"vf" long:"variables-file" description:"path to variables file"`
		AllowOnlyPublishableReleases bool     `long:"allow-only-publishable-releases" description:"include releases that would not be shipped with the tile (development builds)"`
	}
	multiReleaseSourceProvider MultiReleaseSourceProvider
	filesystem                 billy.Filesystem
	logger                     *log.Logger
	loader                     KilnfileLoader
}

func NewUpdateRelease(logger *log.Logger, filesystem billy.Filesystem, multiReleaseSourceProvider MultiReleaseSourceProvider, loader KilnfileLoader) UpdateRelease {
	return UpdateRelease{
		logger:                     logger,
		multiReleaseSourceProvider: multiReleaseSourceProvider,
		filesystem:                 filesystem,
		loader:                     loader,
	}
}

//go:generate counterfeiter -o ./fakes/kilnfile_loader.go --fake-name KilnfileLoader . KilnfileLoader
type KilnfileLoader interface {
	LoadKilnfiles(fs billy.Filesystem, kilnfilePath string, variablesFiles, variables []string) (cargo.Kilnfile, cargo.KilnfileLock, error)
	SaveKilnfileLock(fs billy.Filesystem, kilnfilePath string, lockfile cargo.KilnfileLock) error
}

func (u UpdateRelease) Execute(args []string) error {
	_, err := jhanda.Parse(&u.Options, args)
	if err != nil {
		return err
	}

	kilnfile, kilnfileLock, err := u.loader.LoadKilnfiles(u.filesystem, u.Options.Kilnfile, u.Options.VariablesFiles, u.Options.Variables)
	if err != nil {
		return fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	releaseSource := u.multiReleaseSourceProvider(kilnfile, u.Options.AllowOnlyPublishableReleases)

	u.logger.Println("Searching for the release...")
	remoteRelease, found, err := releaseSource.GetMatchedRelease(release.Requirement{
		Name:            u.Options.Name,
		Version:         u.Options.Version,
		StemcellOS:      kilnfileLock.Stemcell.OS,
		StemcellVersion: kilnfileLock.Stemcell.Version,
	})
	if err != nil {
		return fmt.Errorf("error finding the release: %w", err)
	}
	if !found {
		return fmt.Errorf("couldn't find %q %s in any release source", u.Options.Name, u.Options.Version)
	}

	localRelease, err := releaseSource.DownloadRelease(u.Options.ReleasesDir, remoteRelease, fetcher.DefaultDownloadThreadCount)
	if err != nil {
		return fmt.Errorf("error downloading the release: %w", err)
	}

	var matchingRelease *cargo.ReleaseLock
	for i := range kilnfileLock.Releases {
		if kilnfileLock.Releases[i].Name == u.Options.Name {
			matchingRelease = &kilnfileLock.Releases[i]
			break
		}
	}
	if matchingRelease == nil {
		return fmt.Errorf("no release named %q exists in your Kilnfile.lock", u.Options.Name)
	}

	matchingRelease.Version = localRelease.Version
	matchingRelease.SHA1 = localRelease.SHA1
	matchingRelease.RemoteSource = remoteRelease.SourceID
	matchingRelease.RemotePath = remoteRelease.RemotePath

	err = u.loader.SaveKilnfileLock(u.filesystem, u.Options.Kilnfile, kilnfileLock)
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
