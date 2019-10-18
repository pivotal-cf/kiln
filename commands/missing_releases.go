package commands

import (
	"encoding/json"
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

type MissingReleases struct {
	logger *log.Logger

	releaseSourcesFactory ReleaseSourcesFactory
	localReleaseDirectory LocalReleaseDirectory

	Options struct {
		Kilnfile string `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`

		VariablesFiles      []string `short:"vf" long:"variables-file"        description:"path to variables file"`
		Variables           []string `short:"vr" long:"variable"              description:"variable in key=value format"`
		IncludeTestReleases bool     `short:"t"  long:"include-test-releases" description:"include release sources that are only intended for testing, and not to be used for shipped products"`
	}
}

func NewMissingReleases(logger *log.Logger, releaseSourcesFactory ReleaseSourcesFactory) MissingReleases {
	return MissingReleases{
		logger:                logger,
		releaseSourcesFactory: releaseSourcesFactory,
	}
}

type releaseID struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type missingReleaseJSON struct {
	MissingReleases []releaseID `json:"missing_releases"`
}

func (mr MissingReleases) Execute(args []string) error {
	args, err := jhanda.Parse(&mr.Options, args)
	if err != nil {
		return err
	}

	templateVariablesService := baking.NewTemplateVariablesService()
	templateVariables, err := templateVariablesService.FromPathsAndPairs(mr.Options.VariablesFiles, mr.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}

	kilnfileYAML, err := ioutil.ReadFile(mr.Options.Kilnfile)
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

	var kilnfile cargo.Kilnfile
	err = yaml.Unmarshal(interpolatedMetadata, &kilnfile)
	if err != nil {
		return ConfigFileError{err: err, HumanReadableConfigFileName: "Kilnfile specification " + mr.Options.Kilnfile}
	}

	lockFileName := fmt.Sprintf("%s.lock", mr.Options.Kilnfile)
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
	unsatisfiedReleaseSet := fetcher.NewReleaseSet(kilnfileLock.Releases, kilnfileLock.Stemcell)

	if len(unsatisfiedReleaseSet) > 0 {
		satisfiedReleaseSet, unsatisfiedReleaseSet, err = mr.findMissingReleases(kilnfile, satisfiedReleaseSet, unsatisfiedReleaseSet, kilnfileLock.Stemcell)
		if err != nil {
			return err
		}
	}

	missing := missingReleaseJSON{MissingReleases: make([]releaseID, 0, len(unsatisfiedReleaseSet))}
	for rid, _ := range unsatisfiedReleaseSet {
		missing.MissingReleases = append(missing.MissingReleases, releaseID(rid))
	}
	json, err := json.Marshal(missing)
	if err != nil {
		return err
	}

	fmt.Println(string(json))
	return nil
}

func (mr MissingReleases) findMissingReleases(kilnfile cargo.Kilnfile, satisfiedReleaseSet, unsatisfiedReleaseSet fetcher.ReleaseSet, stemcell cargo.Stemcell) (satisfied, unsatisfied fetcher.ReleaseSet, err error) {
	releaseSources := mr.releaseSourcesFactory.ReleaseSources(kilnfile, mr.Options.IncludeTestReleases)
	for _, releaseSource := range releaseSources {
		matchedReleaseSet, err := releaseSource.GetMatchedReleases(unsatisfiedReleaseSet, stemcell)
		if err != nil {
			return nil, nil, err
		}

		unsatisfiedReleaseSet, satisfiedReleaseSet = unsatisfiedReleaseSet.TransferElements(matchedReleaseSet, satisfiedReleaseSet)
	}

	return satisfiedReleaseSet, unsatisfiedReleaseSet, nil
}

func (mr MissingReleases) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Finds releases listed in Kilnfile.lock that cannot be found in any release source",
		ShortDescription: "finds required releases that can't be found",
		Flags:            mr.Options,
	}
}
