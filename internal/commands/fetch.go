package commands

import (
	"fmt"
	"log"
	"os"

	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type FetchReleaseDir struct {
	ReleasesDir string `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
}

type FetchOptions struct {
	flags.Standard
	flags.FetchBakeOptions
	FetchReleaseDir
}

type Fetch struct {
	logger *log.Logger

	multiReleaseSourceProvider MultiReleaseSourceProvider
	localReleaseDirectory      LocalReleaseDirectory
	Options                    FetchOptions
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
	} else {
		f.logger.Println("All releases already downloaded")
	}

	return nil
}

func (f *Fetch) setup(args []string) (cargo.Kilnfile, cargo.KilnfileLock, []component.Local, error) {
	if f.Options.ReleasesDir == "" {
		_, err := flags.LoadWithDefaultFilePaths(&f.Options, args, nil)
		if err != nil {
			return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
		}
	}

	if !f.Options.AllowOnlyPublishableReleases {
		f.logger.Println("Warning: The \"allow-only-publishable-releases\" flag was not set. Some fetched releases may be intended for development/testing only. EXERCISE CAUTION WHEN PUBLISHING A TILE WITH THESE RELEASES!")
	}

	if _, err := os.Stat(f.Options.ReleasesDir); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(f.Options.ReleasesDir, 0o777)
			if err != nil {
				return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
			}
		} else {
			return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, fmt.Errorf("error with releases directory %s: %w", f.Options.ReleasesDir, err)
		}
	}

	kilnfile, kilnfileLock, err := f.Options.LoadKilnfiles(nil, nil)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
	}

	f.logger.Printf("Gathering releases...")
	availableLocalReleaseSet, err := f.localReleaseDirectory.GetLocalReleases(f.Options.ReleasesDir)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
	}

	return kilnfile, kilnfileLock, availableLocalReleaseSet, nil
}

func (f Fetch) downloadMissingReleases(kilnfile cargo.Kilnfile, releaseLocks []cargo.BOSHReleaseTarballLock) ([]component.Local, error) {
	releaseSource := f.multiReleaseSourceProvider(kilnfile, f.Options.AllowOnlyPublishableReleases)

	var downloaded []component.Local

	for _, rl := range releaseLocks {
		remoteRelease := cargo.BOSHReleaseTarballLock{
			Name:         rl.Name,
			Version:      rl.Version,
			RemotePath:   rl.RemotePath,
			RemoteSource: rl.RemoteSource,
		}

		local, err := releaseSource.DownloadRelease(f.Options.ReleasesDir, remoteRelease)
		if err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}

		if local.Lock.SHA1 != rl.SHA1 {
			err = os.Remove(local.LocalPath)
			if err != nil {
				return nil, fmt.Errorf("error deleting bad release file %q: %w", local.LocalPath, err) // untested
			}

			return nil, fmt.Errorf("downloaded release %q had an incorrect SHA1 - expected %q, got %q", local.LocalPath, rl.SHA1, local.Lock.SHA1)
		}

		downloaded = append(downloaded, local)
	}

	return downloaded, nil
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases in Kilnfile.lock from sources and save in releases directory locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}

func partition(releaseLocks []cargo.BOSHReleaseTarballLock, localReleases []component.Local) (intersection []component.Local, missing []cargo.BOSHReleaseTarballLock, extra []component.Local) {
	missing = make([]cargo.BOSHReleaseTarballLock, len(releaseLocks))
	copy(missing, releaseLocks)

nextRelease:
	for _, rel := range localReleases {
		for j, lock := range missing {
			if rel.Lock.Name == lock.Name && rel.Lock.Version == lock.Version && rel.Lock.SHA1 == lock.SHA1 {
				intersection = append(intersection, rel)
				missing = append(missing[:j], missing[j+1:]...)
				continue nextRelease
			} else if rel.Lock.Name == lock.Name && rel.Lock.Version == lock.Version {
				fmt.Printf("Local release: [ %s ] sha mismatch: [ %s ]\n", lock.Name, rel.Lock.SHA1)
			} else if rel.Lock.Name == lock.Name && rel.Lock.SHA1 == lock.SHA1 {
				fmt.Printf("Local release: [ %s ] version mismatch: [ %s ]\n", lock.Name, rel.Lock.Version)
			}
		}

		extra = append(extra, rel)
	}

	return intersection, missing, extra
}
