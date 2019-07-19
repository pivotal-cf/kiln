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

type multipleError []error

func (errs multipleError) Error() string {
	var strs []string
	for _, err := range errs {
		strs = append(strs, err.Error())
	}
	return strings.Join(strs, "; ")
}

type ConfigFileError struct {
	HumanReadableConfigFileName string
	err                         error
}

func (err ConfigFileError) Unwrap() error {
	return err.err
}

func (err ConfigFileError) Error() string {
	return fmt.Sprintf("encountered a configuration file error with %s: %s", err.HumanReadableConfigFileName, err.err.Error())
}

type stringError string

func (str stringError) Error() string {
	return string(str)
}

type ErrorMissingReleases fetcher.ReleaseSet

func (releases ErrorMissingReleases) Error() string {
	var missing []string
	for id, _ := range releases {
		missing = append(missing, fmt.Sprintf("- %s (%s)", id.Name, id.Version))
	}
	return fmt.Sprintf("could not find the following releases\n%s", strings.Join(missing, "\n"))
}

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
		return ConfigFileError{err: err, HumanReadableConfigFileName: "interpolating variable files with assets file"}
	}

	f.logger.Println("getting release information from " + f.Options.AssetsFile)

	var assets cargo.Assets
	err = yaml.Unmarshal(interpolatedMetadata, &assets)
	if err != nil {
		return ConfigFileError{err: err, HumanReadableConfigFileName: "assets specification " + f.Options.AssetsFile}
	}

	f.logger.Println("getting release information from assets.lock")
	assetsLockFileName := fmt.Sprintf("%s.lock", strings.TrimSuffix(f.Options.AssetsFile, filepath.Ext(f.Options.AssetsFile)))
	assetsLockFile, err := os.Open(assetsLockFileName)
	if err != nil {
		return err
	}
	defer assetsLockFile.Close()

	var assetsLock cargo.AssetsLock
	err = yaml.NewDecoder(assetsLockFile).Decode(&assetsLock)
	if err != nil {
		return ConfigFileError{err: err, HumanReadableConfigFileName: "assets lock " + assetsLockFileName}
	}

	availableLocalReleaseSet, err := f.localReleaseDirectory.GetLocalReleases(f.Options.ReleasesDir)
	if err != nil {
		return err
	}
	if err := f.verifyCompiledReleaseStemcell(availableLocalReleaseSet, assetsLock.Stemcell); err != nil {
		return err
	}
	desiredReleaseSet := fetcher.NewReleaseSet(assetsLock)
	extraReleaseSet := availableLocalReleaseSet.Without(desiredReleaseSet)

	err = f.localReleaseDirectory.DeleteExtraReleases(f.Options.ReleasesDir, extraReleaseSet, f.Options.NoConfirm)
	if err != nil {
		f.logger.Println("failed deleting some releases: ", err.Error())
	}

	satisfiedReleaseSet := availableLocalReleaseSet.Without(extraReleaseSet)
	unsatisfiedReleaseSet := desiredReleaseSet.Without(availableLocalReleaseSet)

	if len(unsatisfiedReleaseSet) > 0 {
		f.logger.Printf("Found %d missing releases to download", len(unsatisfiedReleaseSet))

		satisfiedReleaseSet, unsatisfiedReleaseSet, err = f.downloadMissingReleases(assets, satisfiedReleaseSet, unsatisfiedReleaseSet)
		if err != nil {
			return err
		}
	}

	if len(unsatisfiedReleaseSet) > 0 {
		return ErrorMissingReleases(unsatisfiedReleaseSet)
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

func (f Fetch) verifyCompiledReleaseStemcell(localReleases fetcher.ReleaseSet, stemcell cargo.Stemcell) error {
	var errs []error
	for _, release := range localReleases {
		if rel, ok := release.(fetcher.CompiledRelease); ok {
			if rel.StemcellOS != stemcell.OS || rel.StemcellVersion != stemcell.Version {
				errs = append(errs, IncorrectOSError{
					ReleaseName:    rel.ID.Name,
					ReleaseVersion: rel.ID.Version,
					GotOS:          rel.StemcellOS,
					GotOSVersion:   rel.StemcellVersion,
					WantOS:         stemcell.OS,
					WantOSVersion:  stemcell.Version,
				})
			}
		}
	}
	if len(errs) != 0 {
		return multipleError(errs)
	}
	return nil
}

type IncorrectOSError struct {
	ReleaseName, ReleaseVersion string
	WantOS, GotOS               string
	WantOSVersion, GotOSVersion string
}

func (err IncorrectOSError) Error() string {
	return fmt.Sprintf(
		"expected release %s-%s to have been compiled with %s %s but was compiled with %s %s",
		err.ReleaseName,
		err.ReleaseVersion,
		err.WantOS, err.WantOSVersion,
		err.GotOS, err.GotOSVersion,
	)
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
