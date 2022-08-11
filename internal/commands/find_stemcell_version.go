package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/pivnet"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const (
	ErrStemcellOSInfoMustBeValid       = "stemcell os information is missing or invalid"
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

func NewFindStemcellVersion(outLogger *log.Logger, pivnetService *pivnet.Service) *FindStemcellVersion {
	return &FindStemcellVersion{
		outLogger:     outLogger,
		pivnetService: pivnetService,
		FS:            osfs.New(""),
	}
}

func (cmd *FindStemcellVersion) Execute(args []string) error {
	kilnfile, err := cmd.setup(args)
	if err != nil {
		return err
	}

	productSlug := ""

	// Get Stemcell OS and major from Kilnfile
	if kilnfile.Stemcell.OS == "ubuntu-xenial" {
		productSlug = "stemcells-ubuntu-xenial"
	} else if kilnfile.Stemcell.OS == "ubuntu-jammy" {
		productSlug = "stemcells-ubuntu-jammy"
	} else if kilnfile.Stemcell.OS == "windows2019" {
		productSlug = "stemcells-windows-server"
	}

	if productSlug == "" {
		return fmt.Errorf(ErrStemcellOSInfoMustBeValid)
	}

	if kilnfile.Stemcell.Version == "" {
		return fmt.Errorf(ErrStemcellMajorVersionMustBeValid)
	}

	majorVersion, err := ExtractMajorVersion(kilnfile.Stemcell.Version)
	if err != nil {
		return err
	}

	// Get stemcell version from pivnet
	stemcellVersion, err := cmd.pivnetService.StemcellVersion(productSlug, majorVersion)
	if err != nil {
		return err
	}

	stemcellVersionJson, err := json.Marshal(stemcellVersionOutput{
		Version:    stemcellVersion,
		Source:     "Tanzunet",
		RemotePath: TanzuNetRemotePath,
	})
	if err != nil {
		return err
	}

	cmd.outLogger.Println(string(stemcellVersionJson))

	return nil
}

func ExtractMajorVersion(version string) (string, error) {
	_, err := semver.NewConstraint(version)
	if err != nil {
		return "", fmt.Errorf("invalid stemcell constraint in kilnfile: %w", err)
	}

	semVer := strings.Split(version, ".")

	reg, err := regexp.Compile(`[^0-9]+`)
	if err != nil {
		return "", err
	}

	majorVersion := reg.ReplaceAllString(semVer[0], "")

	if majorVersion == "" {
		return "", fmt.Errorf(ErrStemcellMajorVersionMustBeValid)
	}

	return majorVersion, nil
}

func (cmd *FindStemcellVersion) setup(args []string) (cargo.Kilnfile, error) {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return cargo.Kilnfile{}, err
	}

	_, err = flags.LoadFlagsWithDefaults(&cmd.Options, args, cmd.FS.Stat)
	if err != nil {
		return cargo.Kilnfile{}, err
	}

	kilnfile, _, err := cmd.Options.Standard.LoadKilnfiles(cmd.FS, nil)
	if err != nil {
		return cargo.Kilnfile{}, fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	return kilnfile, nil
}

func (cmd *FindStemcellVersion) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Prints the latest stemcell version from Pivnet using the stemcell type listed in the Kilnfile",
		ShortDescription: "prints the latest stemcell version from Pivnet using the stemcell type listed in the Kilnfile",
		Flags:            cmd.Options,
	}
}
