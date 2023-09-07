package commands

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/pivnet"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const (
	ErrStemcellMajorVersionMustBeValid = "stemcell major Version is missing or invalid"
	TanzuNetRemotePath                 = "network.pivotal.io"
)

type FindStemcellVersion struct {
	outLogger     *log.Logger
	pivnetService *pivnet.Service

	Options struct {
		flags.Standard
	}

	FS billy.Filesystem
}

type stemcellVersionOutput struct {
	Version    string `json:"version"`
	Source     string `json:"source"`
	RemotePath string `json:"remote_path"`
}

func NewFindStemcellVersion(outLogger *log.Logger, pivnetService *pivnet.Service) FindStemcellVersion {
	return FindStemcellVersion{
		outLogger:     outLogger,
		pivnetService: pivnetService,
		FS:            osfs.New(""),
	}
}

func (cmd FindStemcellVersion) Execute(args []string) error {
	kilnfile, err := cmd.setup(args)
	if err != nil {
		return err
	}

	productSlug, err := kilnfile.Stemcell.ProductSlug()
	if err != nil {
		return err
	}

	if kilnfile.Stemcell.Version == "" {
		return fmt.Errorf(ErrStemcellMajorVersionMustBeValid)
	}

	// Get stemcell version from pivnet
	stemcellVersions, err := cmd.pivnetService.Releases(productSlug)
	if err != nil {
		return err
	}

	c, err := semver.NewConstraint(kilnfile.Stemcell.Version)
	if err != nil {
		return err
	}

	v, err := findReleaseWithMatchingConstraint(stemcellVersions, c)
	if err != nil {
		return err
	}

	stemcellVersionJson, err := json.Marshal(stemcellVersionOutput{
		Version:    v,
		Source:     "Tanzunet",
		RemotePath: TanzuNetRemotePath,
	})
	if err != nil {
		return err
	}

	cmd.outLogger.Println(string(stemcellVersionJson))

	return nil
}

func (cmd *FindStemcellVersion) setup(args []string) (cargo.Kilnfile, error) {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return cargo.Kilnfile{}, err
	}

	_, err = flags.LoadWithDefaultFilePaths(&cmd.Options, args, cmd.FS.Stat)
	if err != nil {
		return cargo.Kilnfile{}, err
	}

	kilnfile, _, err := cmd.Options.Standard.LoadKilnfiles(cmd.FS, nil)
	if err != nil {
		return cargo.Kilnfile{}, fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	return kilnfile, nil
}

func (cmd FindStemcellVersion) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Prints the latest stemcell version from Pivnet using the stemcell type listed in the Kilnfile",
		ShortDescription: "prints the latest stemcell version from Pivnet using the stemcell type listed in the Kilnfile",
		Flags:            cmd.Options,
	}
}

func findReleaseWithMatchingConstraint(releases []pivnet.Release, c *semver.Constraints) (string, error) {
	var matchingVersion *semver.Version
	for _, release := range releases {
		v, err := semver.NewVersion(release.Version)
		if err != nil {
			return "", err
		}
		if c.Check(v) && (matchingVersion == nil || v.GreaterThan(matchingVersion)) {
			matchingVersion = v
		}
	}
	if matchingVersion == nil {
		return "", fmt.Errorf("no versions matching constraint")
	}
	return matchingVersion.Original(), nil
}
