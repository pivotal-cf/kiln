package commands

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/kiln/release"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/yaml.v2"
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
	releaseDownloaderFactory ReleaseDownloaderFactory
	filesystem               billy.Filesystem
	logger                   *log.Logger
	checksummer              checksumFunc
	loader                   KilnfileLoader
}

//go:generate counterfeiter -o ./fakes/release_downloader_factory.go --fake-name ReleaseDownloaderFactory . ReleaseDownloaderFactory
type ReleaseDownloaderFactory interface {
	ReleaseDownloader(*log.Logger, cargo.Kilnfile, bool) (ReleaseDownloader, error)
}

//go:generate counterfeiter -o ./fakes/release_downloader.go --fake-name ReleaseDownloader . ReleaseDownloader
type ReleaseDownloader interface {
	DownloadRelease(downloadDir string, requirement release.ReleaseRequirement) (release.LocalRelease, string, string, error)
}

type checksumFunc func(path string, fs billy.Filesystem) (string, error)

func NewUpdateRelease(logger *log.Logger, filesystem billy.Filesystem, releaseDownloaderFactory ReleaseDownloaderFactory, checksummer checksumFunc, loader KilnfileLoader) UpdateRelease {
	return UpdateRelease{
		logger:                   logger,
		releaseDownloaderFactory: releaseDownloaderFactory,
		filesystem:               filesystem,
		checksummer:              checksummer,
		loader:                   loader,
	}
}

//go:generate counterfeiter -o ./fakes/kilnfile_loader.go --fake-name KilnfileLoader . KilnfileLoader
type KilnfileLoader interface {
	LoadKilnfiles(fs billy.Filesystem, kilnfilePath string, variablesFiles, variables []string) (cargo.Kilnfile, cargo.KilnfileLock, error)
}

func (u UpdateRelease) Execute(args []string) error {
	_, err := jhanda.Parse(&u.Options, args)
	if err != nil {
		return err
	}

	kilnfile, kilnfileLock, err := u.loader.LoadKilnfiles(u.filesystem, u.Options.Kilnfile, u.Options.VariablesFiles, u.Options.Variables)
	kilnfileLockPath := fmt.Sprintf("%s.lock", u.Options.Kilnfile)
	if err != nil {
		return fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	releaseDownloader, err := u.releaseDownloaderFactory.ReleaseDownloader(u.logger, kilnfile, u.Options.AllowOnlyPublishableReleases)
	if err != nil {
		return fmt.Errorf("error creating ReleaseDownloader: %w", err)
	}

	u.logger.Println("Searching for the release...")
	localRelease, remoteSource, remotePath, err := releaseDownloader.DownloadRelease(u.Options.ReleasesDir, release.ReleaseRequirement{
		Name:            u.Options.Name,
		Version:         u.Options.Version,
		StemcellOS:      kilnfileLock.Stemcell.OS,
		StemcellVersion: kilnfileLock.Stemcell.Version,
	})
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

	sha, err := u.checksummer(localRelease.LocalPath, u.filesystem)
	if err != nil {
		return fmt.Errorf("error while calculating release checksum: %w", err)
	}

	matchingRelease.Version = u.Options.Version
	matchingRelease.SHA1 = sha
	matchingRelease.RemoteSource = remoteSource
	matchingRelease.RemotePath = remotePath

	updatedLockFileYAML, err := yaml.Marshal(kilnfileLock)
	if err != nil {
		return fmt.Errorf("error marshaling the Kilnfile.lock: %w", err) // untestable
	}

	lockFile, err := u.filesystem.Create(kilnfileLockPath) // overwrites the file
	if err != nil {
		return fmt.Errorf("error reopening the Kilnfile.lock for writing: %w", err)
	}

	_, err = lockFile.Write(updatedLockFileYAML)
	if err != nil {
		return fmt.Errorf("error writing to Kilnfile.lock: %w", err)
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
