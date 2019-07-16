package commands

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf/kiln/fetcher"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/yaml.v2"
)

const (
	ReleaseName     = "release_name"
	ReleaseVersion  = "release_version"
	StemcellOS      = "stemcell_os"
	StemcellVersion = "stemcell_version"
)

type Fetch struct {
	logger *log.Logger

	releaseSourcesFactory func(cargo.Assets) []ReleaseSource
	localReleaseDirectory LocalReleaseDirectory

	Options struct {
		AssetsFile      string   `short:"a" long:"assets-file" required:"true" description:"path to assets file"`
		VariablesFiles  []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables       []string `short:"vr" long:"variable" description:"variable in key=value format"`
		ReleasesDir     string   `short:"rd" long:"releases-directory" required:"true" description:"path to a directory to download releases into"`
		DownloadThreads int      `short:"dt" long:"download-threads" description:"number of parallel threads to download parts from S3"`
		NoConfirm       bool     `short:"n" long:"no-confirm" description:"non-interactive mode, will delete extra releases in releases dir without prompting"`
	}
}

func NewFetch(logger *log.Logger, releaseSourcesFactory func(cargo.Assets) []ReleaseSource, localReleaseDirectory LocalReleaseDirectory) Fetch {
	return Fetch{
		logger:                logger,
		localReleaseDirectory: localReleaseDirectory,
		releaseSourcesFactory: releaseSourcesFactory,
	}
}

//go:generate counterfeiter -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource
type ReleaseSource interface {
	GetMatchedReleases(assetsLock fetcher.ReleaseSet) (fetcher.ReleaseSet, error)
	DownloadReleases(releasesDir string, matchedS3Objects fetcher.ReleaseSet, downloadThreads int) error
}

//go:generate counterfeiter -o ./fakes/local_release_directory.go --fake-name LocalReleaseDirectory . LocalReleaseDirectory
type LocalReleaseDirectory interface {
	GetLocalReleases(releasesDir string) (fetcher.ReleaseSet, error)
	DeleteExtraReleases(releasesDir string, extraReleases fetcher.ReleaseSet, noConfirm bool) error
	VerifyChecksums(releasesDir string, downloadedRelases fetcher.ReleaseSet, assetsLock cargo.AssetsLock) error
}

func (f Fetch) Execute(args []string) error {
	args, err := jhanda.Parse(&f.Options, args)
	if err != nil {
		return err
	}

	templateVariablesService := baking.NewTemplateVariablesService()
	templateVariables, err := templateVariablesService.FromPathsAndPairs(f.Options.VariablesFiles, f.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}

	assetsYAML, err := ioutil.ReadFile(f.Options.AssetsFile)
	if err != nil {
		return err
	}

	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, assetsYAML)
	if err != nil {
		return err
	}

	f.logger.Println("getting release information from assets.yml")

	var assets cargo.Assets
	err = yaml.Unmarshal(interpolatedMetadata, &assets)
	if err != nil {
		return err
	}

	f.logger.Println("getting release information from assets.lock")
	assetsLockFile, err := os.Open(fmt.Sprintf("%s.lock", strings.TrimSuffix(f.Options.AssetsFile, filepath.Ext(f.Options.AssetsFile))))
	if err != nil {
		return err
	}
	defer assetsLockFile.Close()

	var assetsLock cargo.AssetsLock
	err = yaml.NewDecoder(assetsLockFile).Decode(&assetsLock)
	if err != nil {
		return err
	}

	availableLocalReleaseSet, err := f.localReleaseDirectory.GetLocalReleases(f.Options.ReleasesDir)
	if err != nil {
		return err
	}
	existingReleaseSet := f.ensureStemcellFieldsSet(availableLocalReleaseSet, assetsLock.Stemcell)
	desiredReleaseSet := fetcher.NewReleaseSet(assetsLock)
	extraReleaseSet := existingReleaseSet.Without(desiredReleaseSet)

	err = f.localReleaseDirectory.DeleteExtraReleases(f.Options.ReleasesDir, extraReleaseSet, f.Options.NoConfirm)
	if err != nil {
		f.logger.Println("failed deleting some releases: ", err.Error())
	}

	satisfiedReleaseSet := existingReleaseSet.Without(extraReleaseSet)
	unsatisfiedReleaseSet := desiredReleaseSet.Without(existingReleaseSet)

	if len(unsatisfiedReleaseSet) > 0 {
		f.logger.Printf("Found %d missing releases to download", len(unsatisfiedReleaseSet))

		satisfiedReleaseSet, unsatisfiedReleaseSet, err = f.downloadMissingReleases(assets, satisfiedReleaseSet, unsatisfiedReleaseSet)
		if err != nil {
			return err
		}
	}

	if len(unsatisfiedReleaseSet) > 0 {
		formattedMissingReleases := make([]string, 0)

		for missingRelease := range unsatisfiedReleaseSet {
			formattedMissingReleases = append(
				formattedMissingReleases,
				fmt.Sprintf("%+v", missingRelease),
			)

		}
		return fmt.Errorf("Could not find an exact match for these releases in any of the release sources we checked:\n%s", strings.Join(formattedMissingReleases, "\n"))
	}

	return f.localReleaseDirectory.VerifyChecksums(f.Options.ReleasesDir, satisfiedReleaseSet, assetsLock)
}

func (f Fetch) downloadMissingReleases(assets cargo.Assets, satisfiedReleaseSet, unsatisfiedReleaseSet fetcher.ReleaseSet) (satisfied, unsatisfied fetcher.ReleaseSet, err error) {
	releaseSources := f.releaseSourcesFactory(assets)
	for _, releaseSource := range releaseSources {
		matchedReleaseSet, err := releaseSource.GetMatchedReleases(unsatisfiedReleaseSet)
		if err != nil {
			return nil, nil, err
		}

		err = releaseSource.DownloadReleases(f.Options.ReleasesDir, matchedReleaseSet, f.Options.DownloadThreads)
		if err != nil {
			return nil, nil, err
		}

		unsatisfiedReleaseSet, satisfiedReleaseSet = unsatisfiedReleaseSet.TransferElements(matchedReleaseSet, satisfiedReleaseSet)
	}

	return satisfiedReleaseSet, unsatisfiedReleaseSet, nil
}

func (f Fetch) ensureStemcellFieldsSet(localReleases fetcher.ReleaseSet, stemcell cargo.Stemcell) fetcher.ReleaseSet {
	hydratedLocalReleases := make(fetcher.ReleaseSet)

	for localRelease, path := range localReleases {
		if localRelease.StemcellOS == "" {
			localRelease.StemcellOS = stemcell.OS
			localRelease.StemcellVersion = stemcell.Version
		}

		hydratedLocalReleases[localRelease] = path
	}

	return hydratedLocalReleases
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
