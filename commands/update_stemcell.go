package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/Masterminds/semver"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/yaml.v2"
)

const (
	stemcellSlugWindows = "stemcells-windows-server"
	stemcellSlugXenial  = "stemcells-ubuntu-xenial"
	stemcellSlugTrusty  = "stemcells"
)

// UpdateStemcell wraps the dependencies and flag options for the `kiln update-release` command
type UpdateStemcell struct {
	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" required:"true" description:"path to Kilnfile"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
		PivNetToken    string   `short:"pt" env:"PIVOTAL_NETWORK_API_TOKEN" long:"pivotal-network-token" description:"uaa access token for network.pivotal.io"`
	}
	StemcellsVersionsService interface {
		Versions(string) ([]string, error)
		SetToken(string)
	}
}

// Execute expects a Kilnfile to exist and be passed as a flag
func (update UpdateStemcell) Execute(args []string) error {
	_, err := jhanda.Parse(&update.Options, args)
	if err != nil {
		return err
	}

	update.StemcellsVersionsService.SetToken(update.Options.PivNetToken)

	kilnfileYAML, err := ioutil.ReadFile(update.Options.Kilnfile)
	if err != nil {
		return errors.New("could not read kilnfile")
	}
	var kilnfile cargo.Kilnfile
	if err := yaml.Unmarshal(kilnfileYAML, &kilnfile); err != nil {
		return fmt.Errorf("could not parse yaml in kilnfile: %s", err)
	}

	if kilnfile.Stemcell.OS == "" && kilnfile.Stemcell.Version == "" {
		return fmt.Errorf("stemcell OS (%q) and/or version constraint (%q) are not set", kilnfile.Stemcell.OS, kilnfile.Stemcell.Version)
	}

	kilnfileLockPath := fmt.Sprintf("%s.lock", update.Options.Kilnfile)
	kilnfileLockYAML, err := ioutil.ReadFile(kilnfileLockPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read kilnfile: %s", err)
	}
	templateVariablesService := baking.NewTemplateVariablesService(osfs.New(""))
	templateVariables, err := templateVariablesService.FromPathsAndPairs(update.Options.VariablesFiles, update.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}
	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, kilnfileLockYAML)
	if err != nil {
		return err
	}

	var KilnfileLock cargo.KilnfileLock
	if err := yaml.Unmarshal(interpolatedMetadata, &KilnfileLock); err != nil {
		return fmt.Errorf("could not parse yaml in Kilnfile.lock: %s", err)
	}

	stemcellConstraint, err := semver.NewConstraint(kilnfile.Stemcell.Version)
	if err != nil {
		return fmt.Errorf("stemcell_constraint version error: %s", err)
	}

	var stemcellSlug string
	switch kilnfile.Stemcell.OS {
	case "windows":
		stemcellSlug = stemcellSlugWindows
	case "ubuntu-xenial":
		stemcellSlug = stemcellSlugXenial
	case "ubuntu-trusty":
		stemcellSlug = stemcellSlugTrusty
	default:
		return fmt.Errorf("stemcell_constraint os not supported: %s", kilnfile.Stemcell.OS)
	}

	stemcellVersionsStrings, err := update.StemcellsVersionsService.Versions(stemcellSlug)
	if err != nil {
		return fmt.Errorf("could not get stemcell versions: %s", err)
	}
	stemcellVersions := make([]*semver.Version, 0, len(stemcellVersionsStrings))
	for _, str := range stemcellVersionsStrings {
		ver, err := semver.NewVersion(str)
		if err != nil {
			continue
		}
		if stemcellConstraint.Check(ver) {
			stemcellVersions = append(stemcellVersions, ver)
		}
	}
	sort.Sort(semver.Collection(stemcellVersions))

	if len(stemcellVersions) > 0 {
		KilnfileLock.Stemcell.Version = strings.TrimSuffix(stemcellVersions[len(stemcellVersions)-1].String(), ".0")
	}
	KilnfileLock.Stemcell.OS = kilnfile.Stemcell.OS

	os.Remove(kilnfileLockPath)
	lockFile, err := os.Create(kilnfileLockPath)
	if err != nil {
		return err
	}

	updatedLockFileYAML, err := yaml.Marshal(KilnfileLock)
	if err != nil {
		return err
	}
	lockFile.Write(updatedLockFileYAML)
	return nil
}

// Usage implements the Usage part of the jhanda.Command interface
func (update UpdateStemcell) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "updates stemcell_criteria in Kilnfile.lock",
		ShortDescription: "updates stemcell_criteria",
		Flags:            update.Options,
	}
}
