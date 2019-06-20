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

	releaseSources        []ReleaseSource
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

func NewFetch(logger *log.Logger, releaseSources []ReleaseSource, localReleaseDirectory LocalReleaseDirectory) Fetch {
	return Fetch{
		logger:                logger,
		releaseSources:        releaseSources,
		localReleaseDirectory: localReleaseDirectory,
	}
}

//go:generate counterfeiter -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource
type ReleaseSource interface {
	GetMatchedReleases(assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, []cargo.CompiledRelease, error)
	DownloadReleases(releasesDir string, matchedS3Objects map[cargo.CompiledRelease]string, downloadThreads int) error
	Configure(cargo.Assets)
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

	workingAssetsLock := assetsLock

	allMatchedObjects := make(map[cargo.CompiledRelease]string)
	var extraReleases map[cargo.CompiledRelease]string
	var missingReleases []cargo.CompiledRelease

	for _, releaseSource := range f.releaseSources {
		releaseSource.Configure(assets)

		var (
			matchedObjects map[cargo.CompiledRelease]string
			err            error
		)

		localReleases, err := f.localReleaseDirectory.GetLocalReleases(f.Options.ReleasesDir)
		if err != nil {
			return err
		}
		localReleaseSet := f.hydrateLocalReleases(localReleases, assetsLock)

		matchedObjects, missingReleases, err = releaseSource.GetMatchedReleases(workingAssetsLock)
		if err != nil {
			return err
		}
		for k, v := range matchedObjects {
			allMatchedObjects[k] = v
		}

		f.logger.Printf("found %d remote releases", len(matchedObjects))
		var missingLocalReleases map[cargo.CompiledRelease]string
		missingLocalReleases, extraReleases = f.getMissingReleases(allMatchedObjects, localReleaseSet)
		releaseSource.DownloadReleases(f.Options.ReleasesDir, missingLocalReleases, f.Options.DownloadThreads)

		// remove matched releases from workingAssetsLock
		for compiledRelease, _ := range matchedObjects {
			for i, release := range workingAssetsLock.Releases {
				if release.Name == compiledRelease.Name && release.Version == compiledRelease.Version {
					workingAssetsLock.Releases = append(workingAssetsLock.Releases[:i], workingAssetsLock.Releases[i+1:]...)
				}
			}
		}
	}

	// Reconcile releases
	f.localReleaseDirectory.DeleteExtraReleases(f.Options.ReleasesDir, extraReleases, f.Options.NoConfirm)

	if len(missingReleases) > 0 {
		formattedMissingReleases := make([]string, 0)

		for _, missingRelease := range missingReleases {
			formattedMissingReleases = append(
				formattedMissingReleases,
				fmt.Sprintf("%+v", missingRelease),
			)

		}
		return fmt.Errorf("Could not find the following releases:\n%s", strings.Join(formattedMissingReleases, "\n"))
	}

	return f.localReleaseDirectory.VerifyChecksums(f.Options.ReleasesDir, extraReleases, assetsLock)
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
	remoteReleasesCopy := make(map[cargo.CompiledRelease]string)
	localReleasesCopy := make(map[cargo.CompiledRelease]string)

	for k, v := range remoteReleases {
		remoteReleasesCopy[k] = v
	}
	for k, v := range localReleases {
		localReleasesCopy[k] = v
	}
	desiredReleases := make(map[cargo.CompiledRelease]string)

	for key, value := range remoteReleasesCopy {
		desiredReleases[key] = value
	}

	for remoteRelease, _ := range remoteReleasesCopy {
		if _, ok := localReleases[remoteRelease]; ok {
			delete(remoteReleasesCopy, remoteRelease)
		}
	}

	for localRelease, _ := range localReleasesCopy {
		if _, ok := desiredReleases[localRelease]; ok {
			delete(localReleasesCopy, localRelease)
		}
	}

	// first return value is releases missing from local. second return value is extra releases present locally.
	return remoteReleasesCopy, localReleasesCopy
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
