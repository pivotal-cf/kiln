package commands

import (
	"fmt"
	cargo2 "github.com/pivotal-cf/kiln/pkg/cargo"
	release2 "github.com/pivotal-cf/kiln/pkg/release"
	"log"
	"os"

	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/kiln/fetcher"

	"github.com/pivotal-cf/jhanda"
)

type Fetch struct {
	logger *log.Logger

	multiReleaseSourceProvider MultiReleaseSourceProvider
	localReleaseDirectory      LocalReleaseDirectory

	Options struct {
		Kilnfile    string `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		ReleasesDir string `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`

		VariablesFiles               []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables                    []string `short:"vr" long:"variable" description:"variable in key=value format"`
		DownloadThreads              int      `short:"dt" long:"download-threads" description:"number of parallel threads to download parts from S3"`
		NoConfirm                    bool     `short:"n" long:"no-confirm" description:"non-interactive mode, will delete extra releases in releases dir without prompting"`
		AllowOnlyPublishableReleases bool     `long:"allow-only-publishable-releases" description:"include releases that would not be shipped with the tile (development builds)"`
	}
}

//go:generate counterfeiter -o ./fakes/multi_release_source_provider.go --fake-name MultiReleaseSourceProvider . MultiReleaseSourceProvider
type MultiReleaseSourceProvider func(cargo2.Kilnfile, bool) fetcher.MultiReleaseSource

func NewFetch(logger *log.Logger, multiReleaseSourceProvider MultiReleaseSourceProvider, localReleaseDirectory LocalReleaseDirectory) Fetch {
	return Fetch{
		logger:                     logger,
		localReleaseDirectory:      localReleaseDirectory,
		multiReleaseSourceProvider: multiReleaseSourceProvider,
	}
}

//go:generate counterfeiter -o ./fakes/local_release_directory.go --fake-name LocalReleaseDirectory . LocalReleaseDirectory
type LocalReleaseDirectory interface {
	GetLocalReleases(releasesDir string) ([]release2.Local, error)
	DeleteExtraReleases(extraReleases []release2.Local, noConfirm bool) error
}

func (f Fetch) Execute(args []string) error {
	kilnfile, kilnfileLock, availableLocalReleaseSet, err := f.setup(args)
	if err != nil {
		return err
	}

	localReleases, missingReleases, extraReleases := partition(kilnfileLock.Releases, availableLocalReleaseSet)

	err = f.localReleaseDirectory.DeleteExtraReleases(extraReleases, f.Options.NoConfirm)
	if err != nil {
		f.logger.Println("failed deleting some releases: ", err.Error())
	}

	if len(missingReleases) > 0 {
		f.logger.Printf("Found %d missing releases to download", len(missingReleases))

		downloadedReleases, err := f.downloadMissingReleases(kilnfile, missingReleases)
		if err != nil {
			return err
		}

		localReleases = append(localReleases, downloadedReleases...)
	}

	return nil
}

func (f *Fetch) setup(args []string) (cargo2.Kilnfile, cargo2.KilnfileLock, []release2.Local, error) {
	args, err := jhanda.Parse(&f.Options, args)

	if err != nil {
		return cargo2.Kilnfile{}, cargo2.KilnfileLock{}, nil, err
	}
	if !f.Options.AllowOnlyPublishableReleases {
		f.logger.Println("WARNING - the \"allow-only-publishable-releases\" flag was not set. Some fetched releases may be intended for development/testing only.\nEXERCISE CAUTION WHEN PUBLISHING A TILE WITH THESE RELEASES!")
	}
	if _, err := os.Stat(f.Options.ReleasesDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(f.Options.ReleasesDir, 0777)
		} else {
			return cargo2.Kilnfile{}, cargo2.KilnfileLock{}, nil, fmt.Errorf("error with releases directory %s: %s", f.Options.ReleasesDir, err)
		}
	}
	kilnfile, kilnfileLock, err := cargo2.KilnfileLoader{}.LoadKilnfiles(osfs.New(""), f.Options.Kilnfile, f.Options.VariablesFiles, f.Options.Variables)
	if err != nil {
		return cargo2.Kilnfile{}, cargo2.KilnfileLock{}, nil, err
	}

	availableLocalReleaseSet, err := f.localReleaseDirectory.GetLocalReleases(f.Options.ReleasesDir)
	if err != nil {
		return cargo2.Kilnfile{}, cargo2.KilnfileLock{}, nil, err
	}

	return kilnfile, kilnfileLock, availableLocalReleaseSet, nil
}

func (f Fetch) downloadMissingReleases(kilnfile cargo2.Kilnfile, releaseLocks []cargo2.ReleaseLock) ([]release2.Local, error) {
	releaseSource := f.multiReleaseSourceProvider(kilnfile, f.Options.AllowOnlyPublishableReleases)

	var downloaded []release2.Local

	for _, rl := range releaseLocks {
		remoteRelease := release2.Remote{
			ID:         release2.ID{Name: rl.Name, Version: rl.Version},
			RemotePath: rl.RemotePath,
			SourceID:   rl.RemoteSource,
		}

		local, err := releaseSource.DownloadRelease(f.Options.ReleasesDir, remoteRelease, f.Options.DownloadThreads)
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

func partition(releaseLocks []cargo2.ReleaseLock, localReleases []release2.Local) (intersection []release2.Local, missing []cargo2.ReleaseLock, extra []release2.Local) {
	missing = make([]cargo2.ReleaseLock, len(releaseLocks))
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
