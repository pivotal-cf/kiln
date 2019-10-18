package commands

import (
	"fmt"
	"github.com/pivotal-cf/kiln/fetcher"
	"io/ioutil"
	"log"
	"os"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/yaml.v2"
)

type GetRelease struct {
	logger *log.Logger

	releaseSourcesFactory ReleaseSourcesFactory

	Options struct {
		Kilnfile    string `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`

		ReleaseName string `required:"true" short:"r" long:"release-name" description:"the name of the release to download"`
		ReleaseVersion string `required:"true" short:"v" long:"release-version" description:"the version of the release to download"`
		VariablesFiles      []string `short:"vf" long:"variables-file"        description:"path to variables file"`
		Variables           []string `short:"vr" long:"variable"              description:"variable in key=value format"`
		DownloadThreads     int      `short:"dt" long:"download-threads"      description:"number of parallel threads to download parts from S3"`
		IncludeTestReleases bool     `short:"t"  long:"include-test-releases" description:"include release sources that are only intended for testing, and not to be used for shipped products"`
		ReleasesDir string `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
	}
}

func NewGetRelease(logger *log.Logger, releaseSourcesFactory ReleaseSourcesFactory) GetRelease {
	return GetRelease{
		logger:                logger,
		releaseSourcesFactory: releaseSourcesFactory,
	}
}

func (gr GetRelease) Execute(args []string) error {
	args, err := jhanda.Parse(&gr.Options, args)
	if err != nil {
		return err
	}

	if _, err := os.Stat(gr.Options.ReleasesDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(gr.Options.ReleasesDir, 0777)
		} else {
			return fmt.Errorf("error with releases directory %s: %s", gr.Options.ReleasesDir, err)
		}
	}

	templateVariablesService := baking.NewTemplateVariablesService()
	templateVariables, err := templateVariablesService.FromPathsAndPairs(gr.Options.VariablesFiles, gr.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}

	kilnfileYAML, err := ioutil.ReadFile(gr.Options.Kilnfile)
	if err != nil {
		return err
	}

	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, kilnfileYAML)
	if err != nil {
		return ConfigFileError{err: err, HumanReadableConfigFileName: "interpolating variable files with Kilnfile"}
	}

	gr.logger.Println("getting release information from " + gr.Options.Kilnfile)

	var kilnfile cargo.Kilnfile
	err = yaml.Unmarshal(interpolatedMetadata, &kilnfile)
	if err != nil {
		return ConfigFileError{err: err, HumanReadableConfigFileName: "Kilnfile specification " + gr.Options.Kilnfile}
	}

	gr.logger.Println("getting release information from Kilnfile.lock")
	lockFileName := fmt.Sprintf("%s.lock", gr.Options.Kilnfile)
	lockFile, err := os.Open(lockFileName)
	if err != nil {
		return err
	}
	defer lockFile.Close()

	var kilnfileLock cargo.KilnfileLock
	err = yaml.NewDecoder(lockFile).Decode(&kilnfileLock)
	if err != nil {
		return ConfigFileError{err: err, HumanReadableConfigFileName: "Kilnfile.lock " + lockFileName}
	}
	var satisfiedReleaseSet fetcher.ReleaseSet
	unsatisfiedReleaseSet := fetcher.NewReleaseSet([]cargo.Release{{Name: gr.Options.ReleaseName, Version: gr.Options.ReleaseVersion}}, kilnfileLock.Stemcell)

	if len(unsatisfiedReleaseSet) > 0 {
		gr.logger.Printf("Found %d missing releases to download", len(unsatisfiedReleaseSet))

		satisfiedReleaseSet, unsatisfiedReleaseSet, err = gr.downloadMissingReleases(kilnfile, satisfiedReleaseSet, unsatisfiedReleaseSet, kilnfileLock.Stemcell)
		if err != nil {
			return err
		}
	}

	if len(unsatisfiedReleaseSet) > 0 {
		return ErrorMissingReleases(unsatisfiedReleaseSet)
	}

	return nil
}

func (gr GetRelease) downloadMissingReleases(kilnfile cargo.Kilnfile, satisfiedReleaseSet, unsatisfiedReleaseSet fetcher.ReleaseSet, stemcell cargo.Stemcell) (satisfied, unsatisfied fetcher.ReleaseSet, err error) {
	releaseSources := gr.releaseSourcesFactory.ReleaseSources(kilnfile, gr.Options.IncludeTestReleases)
	for _, releaseSource := range releaseSources {
		matchedReleaseSet, err := releaseSource.GetMatchedReleases(unsatisfiedReleaseSet, stemcell)
		if err != nil {
			return nil, nil, err
		}

		err = releaseSource.DownloadReleases(gr.Options.ReleasesDir, matchedReleaseSet, gr.Options.DownloadThreads)
		if err != nil {
			return nil, nil, err
		}

		unsatisfiedReleaseSet, satisfiedReleaseSet = unsatisfiedReleaseSet.TransferElements(matchedReleaseSet, satisfiedReleaseSet)
	}

	return satisfiedReleaseSet, unsatisfiedReleaseSet, nil
}

func (gr GetRelease) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Download the specified release from our release sources",
		ShortDescription: "download a specific release",
		Flags:            gr.Options,
	}
}
