package commands

import (
	"fmt"
	"github.com/pivotal-cf/kiln/pkg/component"
	"log"
	"os"

	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Fetch struct {
	logger *log.Logger

	multiReleaseSourceProvider MultiReleaseSourceProvider
	localReleaseDirectory      LocalReleaseDirectory

	Options struct {
		flags.Standard

		ReleasesDir string `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`

		DownloadThreads              int  `short:"dt" long:"download-threads" description:"number of parallel threads to download parts from S3"`
		NoConfirm                    bool `short:"n" long:"no-confirm" description:"non-interactive mode, will delete extra releases in releases dir without prompting"`
		AllowOnlyPublishableReleases bool `long:"allow-only-publishable-releases" description:"include releases that would not be shipped with the tile (development builds)"`
	}
}

//counterfeiter:generate -o ./fakes/multi_release_source_provider.go --fake-name MultiReleaseSourceProvider . MultiReleaseSourceProvider
type MultiReleaseSourceProvider func(cargo.Kilnfile, bool) component.MultiReleaseSource

func NewFetch(logger *log.Logger, multiReleaseSourceProvider MultiReleaseSourceProvider, localReleaseDirectory LocalReleaseDirectory) Fetch {
	return Fetch{
		logger:                     logger,
		localReleaseDirectory:      localReleaseDirectory,
		multiReleaseSourceProvider: multiReleaseSourceProvider,
	}
}

//counterfeiter:generate -o ./fakes/local_release_directory.go --fake-name LocalReleaseDirectory . LocalReleaseDirectory
type LocalReleaseDirectory interface {
	GetLocalReleases(releasesDir string) ([]component.Local, error)
	DeleteExtraReleases(extraReleases []component.Local, noConfirm bool) error
}

func (f Fetch) Execute(args []string) error {
	kilnfile, kilnfileLock, availableLocalReleaseSet, err := f.setup(args)
	if err != nil {
		return err
	}

	_, missingReleases, extraReleases := partition(kilnfileLock.Releases, availableLocalReleaseSet)

	err = f.localReleaseDirectory.DeleteExtraReleases(extraReleases, f.Options.NoConfirm)
	if err != nil {
		f.logger.Println("failed deleting some releases: ", err.Error())
	}

	if len(missingReleases) > 0 {
		f.logger.Printf("Found %d missing releases to download", len(missingReleases))

		_, err := f.downloadMissingReleases(kilnfile, missingReleases)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *Fetch) setup(args []string) (cargo.Kilnfile, cargo.KilnfileLock, []component.Local, error) {
	_, err := flags.LoadFlagsWithDefaults(&f.Options, args, nil)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
	}
	if !f.Options.AllowOnlyPublishableReleases {
		f.logger.Println("WARNING - the \"allow-only-publishable-releases\" flag was not set. Some fetched releases may be intended for development/testing only.\nEXERCISE CAUTION WHEN PUBLISHING A TILE WITH THESE RELEASES!")
	}
	if _, err := os.Stat(f.Options.ReleasesDir); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(f.Options.ReleasesDir, 0777)
			if err != nil {
				return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
			}
		} else {
			return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, fmt.Errorf("error with releases directory %s: %s", f.Options.ReleasesDir, err)
		}
	}

	kilnfile, kilnfileLock, err := f.Options.LoadKilnfiles(nil, nil)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
	}

	availableLocalReleaseSet, err := f.localReleaseDirectory.GetLocalReleases(f.Options.ReleasesDir)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
	}

	return kilnfile, kilnfileLock, availableLocalReleaseSet, nil
}

func (f Fetch) downloadMissingReleases(kilnfile cargo.Kilnfile, releaseLocks []cargo.ComponentLock) ([]component.Local, error) {
	releaseSource := f.multiReleaseSourceProvider(kilnfile, f.Options.AllowOnlyPublishableReleases)

	// f.Options.DownloadThreads

	var downloaded []component.Local

	for _, rl := range releaseLocks {
		remoteRelease := component.Lock{
			Name:         rl.Name,
			Version:      rl.Version,
			RemotePath:   rl.RemotePath,
			RemoteSource: rl.RemoteSource,
		}

		local, err := releaseSource.DownloadRelease(f.Options.ReleasesDir, remoteRelease)
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

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in Kilnfile.lock from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}

func partition(releaseLocks []cargo.ComponentLock, localReleases []component.Local) (intersection []component.Local, missing []cargo.ComponentLock, extra []component.Local) {
	missing = make([]cargo.ComponentLock, len(releaseLocks))
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
