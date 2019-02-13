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

	downloader     Downloader
	releaseMatcher ReleaseMatcher

	Options struct {
		AssetsFile      string   `short:"a" long:"assets-file" required:"true" description:"path to assets file"`
		VariablesFiles  []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables       []string `short:"vr" long:"variable" description:"variable in key=value format"`
		ReleasesDir     string   `short:"rd" long:"releases-directory" required:"true" description:"path to a directory to download releases into"`
		DownloadThreads int      `short:"dt" long:"download-threads" description:"number of parallel threads to download parts from S3"`
	}
}

func NewFetch(logger *log.Logger, downloader Downloader, releaseMatcher ReleaseMatcher) Fetch {
	return Fetch{
		logger:         logger,
		downloader:     downloader,
		releaseMatcher: releaseMatcher,
	}
}

//go:generate counterfeiter -o ./fakes/downloader.go --fake-name Downloader . Downloader
type Downloader interface {
	DownloadReleases(releasesDir string, compiledReleases cargo.CompiledReleases, matchedS3Objects map[cargo.CompiledRelease]string, downloadThreads int) error
}

//go:generate counterfeiter -o ./fakes/release_matcher.go --fake-name ReleaseMatcher . ReleaseMatcher
type ReleaseMatcher interface {
	GetMatchedReleases(compiledReleases cargo.CompiledReleases, assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, error)
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

	matchedS3Objects, err := f.releaseMatcher.GetMatchedReleases(assets.CompiledReleases, assetsLock)
	if err != nil {
		return err
	}

	f.logger.Printf("number of matched S3 objects: %d\n", len(matchedS3Objects))

	return f.downloader.DownloadReleases(f.Options.ReleasesDir, assets.CompiledReleases, matchedS3Objects, f.Options.DownloadThreads)
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
