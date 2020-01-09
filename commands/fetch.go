package commands

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pivotal-cf/kiln/release"

	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/kiln/fetcher"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

type ErrorMissingReleases release.ReleaseRequirementSet

func (releases ErrorMissingReleases) Error() string {
	var missing []string
	for id, _ := range releases {
		missing = append(missing, fmt.Sprintf("- %s (%s)", id.Name, id.Version))
	}
	return fmt.Sprintf("could not find the following releases\n%s", strings.Join(missing, "\n"))
}

type Fetch struct {
	logger *log.Logger

	releaseSourcesFactory ReleaseSourcesFactory
	localReleaseDirectory LocalReleaseDirectory

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

//go:generate counterfeiter -o ./fakes/release_sources_factory.go --fake-name ReleaseSourcesFactory . ReleaseSourcesFactory
type ReleaseSourcesFactory interface {
	ReleaseSources(cargo.Kilnfile, bool) []fetcher.ReleaseSource
}

func NewFetch(logger *log.Logger, releaseSourcesFactory ReleaseSourcesFactory, localReleaseDirectory LocalReleaseDirectory) Fetch {
	return Fetch{
		logger:                logger,
		localReleaseDirectory: localReleaseDirectory,
		releaseSourcesFactory: releaseSourcesFactory,
	}
}

//go:generate counterfeiter -o ./fakes/local_release_directory.go --fake-name LocalReleaseDirectory . LocalReleaseDirectory
type LocalReleaseDirectory interface {
	GetLocalReleases(releasesDir string) (release.ReleaseWithLocationSet, error)
	DeleteExtraReleases(extraReleases release.LocalReleaseSet, noConfirm bool) error
	VerifyChecksums(downloadedReleases release.LocalReleaseSet, kilnfileLock cargo.KilnfileLock) error
}

func (f Fetch) Execute(args []string) error {
	kilnfile, kilnfileLock, availableLocalReleaseSet, err := f.setup(args)
	if err != nil {
		return err
	}

	desiredReleaseSet := release.NewReleaseRequirementSet(kilnfileLock)
	satisfiedReleaseSet, unsatisfiedReleaseSet, extraReleaseSet := desiredReleaseSet.Partition(availableLocalReleaseSet)

	err = f.localReleaseDirectory.DeleteExtraReleases(extraReleaseSet, f.Options.NoConfirm)
	if err != nil {
		f.logger.Println("failed deleting some releases: ", err.Error())
	}

	if len(unsatisfiedReleaseSet) > 0 {
		f.logger.Printf("Found %d missing releases to download", len(unsatisfiedReleaseSet))

		satisfiedReleaseSet, unsatisfiedReleaseSet, err = f.downloadMissingReleases(kilnfile, satisfiedReleaseSet, unsatisfiedReleaseSet, kilnfileLock.Stemcell)
		if err != nil {
			return err
		}
	}

	if len(unsatisfiedReleaseSet) > 0 {
		return ErrorMissingReleases(unsatisfiedReleaseSet)
	}

	return f.localReleaseDirectory.VerifyChecksums(satisfiedReleaseSet, kilnfileLock)
}

func (f *Fetch) setup(args []string) (cargo.Kilnfile, cargo.KilnfileLock, release.ReleaseWithLocationSet, error) {
	args, err := jhanda.Parse(&f.Options, args)

	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
	}
	if !f.Options.AllowOnlyPublishableReleases {
		f.logger.Println("WARNING - the \"allow-only-publishable-releases\" flag was not set. Some fetched releases may be intended for development/testing only.\nEXERCISE CAUTION WHEN PUBLISHING A TILE WITH THESE RELEASES!")
	}
	if _, err := os.Stat(f.Options.ReleasesDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(f.Options.ReleasesDir, 0777)
		} else {
			return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, fmt.Errorf("error with releases directory %s: %s", f.Options.ReleasesDir, err)
		}
	}
	kilnfile, kilnfileLock, err := cargo.KilnfileLoader{}.LoadKilnfiles(osfs.New(""), f.Options.Kilnfile, f.Options.VariablesFiles, f.Options.Variables)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
	}

	availableLocalReleaseSet, err := f.localReleaseDirectory.GetLocalReleases(f.Options.ReleasesDir)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, nil, err
	}

	return kilnfile, kilnfileLock, availableLocalReleaseSet, nil
}

func (f Fetch) downloadMissingReleases(kilnfile cargo.Kilnfile, satisfiedReleaseSet release.LocalReleaseSet, unsatisfiedReleaseSet release.ReleaseRequirementSet, stemcell cargo.Stemcell) (satisfied release.LocalReleaseSet, unsatisfied release.ReleaseRequirementSet, err error) {
	releaseSources := f.releaseSourcesFactory.ReleaseSources(kilnfile, f.Options.AllowOnlyPublishableReleases)
	for _, releaseSource := range releaseSources {
		if len(unsatisfiedReleaseSet) == 0 {
			break
		}
		remoteReleases, err := releaseSource.GetMatchedReleases(unsatisfiedReleaseSet)
		if err != nil {
			return nil, nil, err
		}

		//TODO: make get and download release functions singular, why? We do not need to download all at once because under the cover we are downloading in serially.

		rrs := make([]release.RemoteRelease, 0, len(remoteReleases))
		for _, r := range remoteReleases {
			rrs = append(rrs, release.RemoteRelease{ReleaseID: r.ReleaseID(), RemotePath: r.RemotePath()})
		}

		localReleases, err := releaseSource.DownloadReleases(f.Options.ReleasesDir, rrs, f.Options.DownloadThreads)
		if err != nil {
			return nil, nil, err
		}

		satisfiedReleaseSet = satisfiedReleaseSet.With(localReleases)
		unsatisfiedReleaseSet = unsatisfiedReleaseSet.WithoutReleases(localReleases.ReleaseIDs())
	}

	return satisfiedReleaseSet, unsatisfiedReleaseSet, nil
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in Kilnfile.lock from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
