package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

const (
	ErrStemcellOSInfoMustBeValid       = "Stemcell OS information is missing or invalid"
	ErrStemcellMajorVersionMustBeValid = "Stemcell Major Version is missing or invalid"
	TanzunetRemotePath                 = "network.pivotal.io"
)

type FindStemcellVersion struct {
	outLogger     *log.Logger
	pivnetService *fetcher.Pivnet

	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
	}
}

type stemcellVersionOutput struct {
	Version    string `json:"version"`
	Source     string `json:"source"`
	RemotePath string `json:"remote_path"`
}

func NewFindStemcellVersion(outLogger *log.Logger, pivnetService *fetcher.Pivnet) FindStemcellVersion {
	return FindStemcellVersion{
		outLogger:     outLogger,
		pivnetService: pivnetService,
	}
}

func (cmd FindStemcellVersion) Execute(args []string) error {
	kilnfile, err := cmd.setup(args)

	if err != nil {
		return err
	}

	var productSlug = ""

	//Get Stemcell OS and major from Kilnfile
	if kilnfile.Stemcell.OS == "ubuntu-xenial" {
		productSlug = "stemcells-ubuntu-xenial"
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

	//Get stemcell version from pivnet
	stemcellVersion, err := cmd.pivnetService.StemcellVersion(productSlug, majorVersion)

	if err != nil {
		return err
	}

	stemcellVersionJson, err := json.Marshal(stemcellVersionOutput{
		Version:    stemcellVersion,
		Source:     "Tanzunet",
		RemotePath: TanzunetRemotePath,
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
		return "", fmt.Errorf("Invalid stemcell constraint in kilnfile: %w", err)
	}

	semVer := strings.Split(version, ".")

	reg, err := regexp.Compile("[^0-9]+")

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

	kilnfile, _, err := cargo.KilnfileLoader{}.LoadKilnfiles(osfs.New(""), cmd.Options.Kilnfile, cmd.Options.VariablesFiles, cmd.Options.Variables)
	if err != nil {
		return cargo.Kilnfile{}, err
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
