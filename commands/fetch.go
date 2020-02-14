package commands

import (
	"fmt"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4"
	"log"
	"os"

	"github.com/pivotal-cf/kiln/fetcher"

	"github.com/pivotal-cf/kiln/internal/cargo"
)

type FetchCmd struct {
	ReleasesDir                  string `long:"releases-directory" short:"r"  default:"releases" description:"path to a directory to download releases into"`
	DownloadThreads              int    `long:"download-threads"   short:"t"                     description:"number of parallel threads to download parts from S3"`
	NoConfirm                    bool   `long:"no-confirm"         short:"n"                     description:"non-interactive mode, will delete extra releases in releases dir without prompting"`
	AllowOnlyPublishableReleases bool   `long:"allow-only-publishable-releases"                  description:"include releases that would not be shipped with the tile (development builds)"`
	panicCommand
}

type CommandRunner interface {
	Run([]string) error
}

type panicCommand struct {}
func (panicCommand) Execute(_ []string) error {
	panic("This should never be called")
}

type Dependencies struct {
	Kilnfile              cargo.Kilnfile
	KilnfileLock          cargo.KilnfileLock
	KilnfilePath          string
	KilnfileLockPath      string
	Variables             []string
	VariablesFiles        []string
	ReleaseSourceRepo     fetcher.ReleaseSourceRepo
	LocalReleaseDirectory LocalReleaseDirectory
	OutLogger             *log.Logger
	ErrLogger             *log.Logger
	Filesystem            billy.Filesystem
}

func (f FetchCmd) Runner(deps Dependencies) (CommandRunner, error) {
	return Fetch{
		ReleasesDir: f.ReleasesDir,
		DownloadThreads: f.DownloadThreads,
		NoConfirm: f.NoConfirm,
		AllowOnlyPublishableReleases: f.AllowOnlyPublishableReleases,

		MultiReleaseSourceProvider: deps.ReleaseSourceRepo.MultiReleaseSource,
		Kilnfile: deps.Kilnfile,
		KilnfileLock: deps.KilnfileLock,
		LocalReleaseDirectory: deps.LocalReleaseDirectory,
		Logger: deps.OutLogger,
	}, nil
}

type Fetch struct {
	ReleasesDir                  string
	DownloadThreads              int
	NoConfirm                    bool
	AllowOnlyPublishableReleases bool

	MultiReleaseSourceProvider MultiReleaseSourceProvider
	LocalReleaseDirectory      LocalReleaseDirectory
	Kilnfile                   cargo.Kilnfile
	KilnfileLock               cargo.KilnfileLock
	Logger                     *log.Logger
}

//go:generate counterfeiter -o ./fakes/multi_release_source_provider.go --fake-name MultiReleaseSourceProvider . MultiReleaseSourceProvider
type MultiReleaseSourceProvider func(bool) fetcher.MultiReleaseSource

//go:generate counterfeiter -o ./fakes/local_release_directory.go --fake-name LocalReleaseDirectory . LocalReleaseDirectory
type LocalReleaseDirectory interface {
	GetLocalReleases(releasesDir string) ([]release.Local, error)
	DeleteExtraReleases(extraReleases []release.Local, noConfirm bool) error
}

func (f Fetch) Run(_ []string) error {
	if !f.AllowOnlyPublishableReleases {
		f.Logger.Println("WARNING - the \"allow-only-publishable-releases\" flag was not set. Some fetched releases may be intended for development/testing only.\nEXERCISE CAUTION WHEN PUBLISHING A TILE WITH THESE RELEASES!")
	}

	if _, err := os.Stat(f.ReleasesDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(f.ReleasesDir, 0777)
		} else {
			return fmt.Errorf("error with releases directory %s: %s", f.ReleasesDir, err)
		}
	}

	availableLocalReleaseSet, err := f.LocalReleaseDirectory.GetLocalReleases(f.ReleasesDir)
	if err != nil {
		return err
	}

	localReleases, missingReleases, extraReleases := partition(f.KilnfileLock.Releases, availableLocalReleaseSet)

	err = f.LocalReleaseDirectory.DeleteExtraReleases(extraReleases, f.NoConfirm)
	if err != nil {
		f.Logger.Println("failed deleting some releases: ", err.Error())
	}

	if len(missingReleases) > 0 {
		f.Logger.Printf("Found %d missing releases to download", len(missingReleases))

		downloadedReleases, err := f.downloadMissingReleases(missingReleases)
		if err != nil {
			return err
		}

		localReleases = append(localReleases, downloadedReleases...)
	}

	return nil
}

func (f Fetch) downloadMissingReleases(releaseLocks []cargo.ReleaseLock) ([]release.Local, error) {
	var downloaded []release.Local
	multiReleaseSource := f.MultiReleaseSourceProvider(f.AllowOnlyPublishableReleases)

	for _, rl := range releaseLocks {
		remoteRelease := release.Remote{
			ID:         release.ID{Name: rl.Name, Version: rl.Version},
			RemotePath: rl.RemotePath,
			SourceID:   rl.RemoteSource,
		}

		local, err := multiReleaseSource.DownloadRelease(f.ReleasesDir, remoteRelease, f.DownloadThreads)
		if err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}

		if local.SHA1 != rl.SHA1 {
			err = os.Remove(local.LocalPath)
			if err != nil {
				return nil, fmt.Errorf("error deleting bad release file %q: %w", local.LocalPath, err) // untested
			}

			return nil, fmt.Errorf("downloaded release %q had an incorrect SHA1 - expected %q, got %q", local.LocalPath, rl.SHA1, local.SHA1)
		}

		downloaded = append(downloaded, local)
	}

	return downloaded, nil
}

func partition(releaseLocks []cargo.ReleaseLock, localReleases []release.Local) (intersection []release.Local, missing []cargo.ReleaseLock, extra []release.Local) {
	missing = make([]cargo.ReleaseLock, len(releaseLocks))
	copy(missing, releaseLocks)

nextRelease:
	for _, rel := range localReleases {
		for j, lock := range missing {
			if rel.Name == lock.Name && rel.Version == lock.Version && rel.SHA1 == lock.SHA1 {
				intersection = append(intersection, rel)
				missing = append(missing[:j], missing[j+1:]...)
				continue nextRelease
			}
		}

		extra = append(extra, rel)
	}

	return intersection, missing, extra
}
