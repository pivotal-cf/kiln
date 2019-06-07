package commands

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
	yaml "gopkg.in/yaml.v2"
)

const (
	ReleaseName     = "release_name"
	ReleaseVersion  = "release_version"
	StemcellOS      = "stemcell_os"
	StemcellVersion = "stemcell_version"
)

type Fetch struct {
	logger *log.Logger

	downloader            Downloader
	releaseMatcher        ReleaseMatcher
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

func NewFetch(logger *log.Logger, downloader Downloader, releaseMatcher ReleaseMatcher, localReleaseDirectory LocalReleaseDirectory) Fetch {
	return Fetch{
		logger:                logger,
		downloader:            downloader,
		releaseMatcher:        releaseMatcher,
		localReleaseDirectory: localReleaseDirectory,
	}
}

//go:generate counterfeiter -o ./fakes/downloader.go --fake-name Downloader . Downloader
type Downloader interface {
	DownloadReleases(releasesDir string, compiledReleases cargo.CompiledReleases, matchedS3Objects map[cargo.CompiledRelease]string, downloadThreads int) error
}

//go:generate counterfeiter -o ./fakes/release_matcher.go --fake-name ReleaseMatcher . ReleaseMatcher
type ReleaseMatcher interface {
	GetMatchedReleases(compiledReleases cargo.CompiledReleases, assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, []cargo.CompiledRelease, error)
}

//go:generate counterfeiter -o ./fakes/local_release_directory.go --fake-name LocalReleaseDirectory . LocalReleaseDirectory
type LocalReleaseDirectory interface {
	GetLocalReleases(releasesDir string) (map[cargo.CompiledRelease]string, error)
	DeleteExtraReleases(releasesDir string, extraReleases map[cargo.CompiledRelease]string, noConfirm bool) error
	VerifyChecksums(releasesDir string, downloadedRelases map[cargo.CompiledRelease]string, assetsLock cargo.AssetsLock) error
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

	f.logger.Println("getting S3 information from assets.yml")

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

	//TODO: Add returned slice missingReleases, listing out releases not in S3
	matchedS3Objects, unmatchedObjects, err := f.releaseMatcher.GetMatchedReleases(assets.CompiledReleases, assetsLock)
	if err != nil {
		return err
	}

	if len(unmatchedObjects) > 0 {
		formattedMissingReleases := make([]string, 0)

		for _, missingRelease := range unmatchedObjects {
			formattedMissingReleases = append(
				formattedMissingReleases,
				fmt.Sprintf("%+v", missingRelease,),
			)

		}
		return fmt.Errorf("Expected releases were not matched by the regex:\n%s", strings.Join(formattedMissingReleases, "\n"))
	}

	f.logger.Printf("found %d remote releases", len(matchedS3Objects))

	localReleases, err := f.localReleaseDirectory.GetLocalReleases(f.Options.ReleasesDir)
	if err != nil {
		return err
	}

	localReleaseSet := f.hydrateLocalReleases(localReleases, assetsLock)

	//missingReleases are the releases not in local dir, to be fetched from S3
	missingReleases, extraReleases := f.getMissingReleases(matchedS3Objects, localReleaseSet)

	f.localReleaseDirectory.DeleteExtraReleases(f.Options.ReleasesDir, extraReleases, f.Options.NoConfirm)

	f.logger.Printf("downloading %d objects from S3...", len(missingReleases))

	f.downloader.DownloadReleases(f.Options.ReleasesDir, assets.CompiledReleases, missingReleases, f.Options.DownloadThreads)

	return f.localReleaseDirectory.VerifyChecksums(f.Options.ReleasesDir, missingReleases, assetsLock)
}

func (f Fetch) hydrateLocalReleases(localReleases map[cargo.CompiledRelease]string, assetsLock cargo.AssetsLock) map[cargo.CompiledRelease]string {
	hydratedLocalReleases := make(map[cargo.CompiledRelease]string)

	for localRelease, path := range localReleases {
		if localRelease.StemcellOS == "" {
			localRelease.StemcellOS = assetsLock.Stemcell.OS
			localRelease.StemcellVersion = assetsLock.Stemcell.Version
		}

		hydratedLocalReleases[localRelease] = path
	}

	return hydratedLocalReleases
}

func (f Fetch) getMissingReleases(remoteReleases map[cargo.CompiledRelease]string, localReleases map[cargo.CompiledRelease]string) (map[cargo.CompiledRelease]string, map[cargo.CompiledRelease]string) {
	desiredReleases := make(map[cargo.CompiledRelease]string)

	for key, value := range remoteReleases {
		desiredReleases[key] = value
	}

	for remoteRelease, _ := range remoteReleases {
		if _, ok := localReleases[remoteRelease]; ok {
			delete(remoteReleases, remoteRelease)
		}
	}

	for localRelease, _ := range localReleases {
		if _, ok := desiredReleases[localRelease]; ok {
			delete(localReleases, localRelease)
		}
	}

	// first return value is releases missing from local. second return value is extra releases present locally.
	return remoteReleases, localReleases
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
