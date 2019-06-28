package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/cargo"
	yaml "gopkg.in/yaml.v2"
)

const (
	StemcellSlugWindows = "stemcells-windows-server"
	StemcellSlugXenial  = "stemcells-ubuntu-xenial"
	StemcellSlugTrusty  = "stemcells"
)

// Update wraps the dependancies and flag options for the `kiln update` command
type Update struct {
	Options struct {
		AssetsFile string `short:"a" long:"assets-file" required:"true" description:"path to assets file"`
	}
	StemcellsVersionsService interface {
		Versions(string) ([]string, error)
	}
}

// Execute expects an assets.yaml to exist and be passed as a flag
func (update Update) Execute(args []string) error {
	_, err := jhanda.Parse(&update.Options, args)
	if err != nil {
		return err
	}
	assetsSpecYAML, err := ioutil.ReadFile(update.Options.AssetsFile)
	if err != nil {
		return errors.New("could not read assets-file")
	}
	var assetsSpec cargo.Assets
	if err := yaml.Unmarshal(assetsSpecYAML, &assetsSpec); err != nil {
		return fmt.Errorf("could not parse yaml in assets-file: %s", err)
	}

	assetsLockPath := filepath.Join(filepath.Dir(update.Options.AssetsFile), "assets.lock")
	assetsLockYAML, err := ioutil.ReadFile(assetsLockPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read assets-file: %s", err)
	}
	var assetsLock cargo.AssetsLock
	if err := yaml.Unmarshal(assetsLockYAML, &assetsLock); err != nil {
		return fmt.Errorf("could not parse yaml in assets.lock: %s", err)
	}

	stemcellConstraint, err := semver.NewConstraint(assetsSpec.Stemcell.Version)
	if err != nil {
		return fmt.Errorf("stemcell_constraint version error: %s", err)
	}

	var stemcellSlug string
	switch assetsSpec.Stemcell.OS {
	case "windows":
		stemcellSlug = StemcellSlugWindows
	case "ubuntu-xenial":
		stemcellSlug = StemcellSlugXenial
	case "ubuntu-trusty":
		stemcellSlug = StemcellSlugTrusty
	default:
		return fmt.Errorf("stemcell_constraint os not supported: %s", assetsSpec.Stemcell.OS)
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
		assetsLock.Stemcell.Version = strings.TrimSuffix(stemcellVersions[len(stemcellVersions)-1].String(), ".0")
	}
	assetsLock.Stemcell.OS = assetsSpec.Stemcell.OS

	os.Remove(assetsLockPath)
	assetsLockFile, err := os.Create(assetsLockPath)
	if err != nil {
		return err
	}

	assetsLockUpdatedYAML, err := yaml.Marshal(assetsLock)
	if err != nil {
		return err
	}
	assetsLockFile.Write([]byte(assetsYAMLHeader))
	assetsLockFile.Write(assetsLockUpdatedYAML)
	return nil
}

// Usage implements the Usage part of the jhanda.Command interface
func (update Update) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Updates stemcell_criteria and releases.",
		ShortDescription: "Updates stemcell_criteria and releases",
		Flags:            update.Options,
	}
}

const (
	assetsYAMLHeader = "########### DO NOT EDIT! ############\n" +
		"# This is a machine generated file, #\n" +
		"# update by running `kiln update`   #\n" +
		"#####################################\n" +
		"---\n"
)
