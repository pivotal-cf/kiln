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

	"github.com/pivotal-cf/kiln/internal/commands/options"
	"github.com/pivotal-cf/kiln/internal/component"
)

const (
	ErrStemcellOSInfoMustBeValid       = "stemcell os information is missing or invalid"
	ErrStemcellMajorVersionMustBeValid = "stemcell major version is missing or invalid"
	TanzuNetRemotePath                 = "network.pivotal.io"
)

type FindStemcellVersion struct {
	outLogger     *log.Logger
	pivnetService *component.Pivnet

	Options struct {
		options.Standard
	}

	FS billy.Filesystem
}

type stemcellVersionOutput struct {
	Version    string `json:"version"`
	Source     string `json:"source"`
	RemotePath string `json:"remote_path"`
}

func NewFindStemcellVersion(outLogger *log.Logger, pivnetService *component.Pivnet) FindStemcellVersion {
	return FindStemcellVersion{
		outLogger:     outLogger,
		pivnetService: pivnetService,
		FS:            osfs.New(""),
	}
}

func (f FindStemcellVersion) Execute(args []string) error {
	return Kiln{
		Wrapped: f,
		KilnfileStore: KilnfileStore{
			FS: f.FS,
		},
		StatFn: f.FS.Stat,
	}.Execute(args)
}

func (f FindStemcellVersion) KilnExecute(args []string, parseOpts OptionsParseFunc) error {
	kilnfile, _, _, err := parseOpts(args, &f.Options)
	if err != nil {
		return err
	}

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
	stemcellVersion, err := f.pivnetService.StemcellVersion(productSlug, majorVersion)

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

	f.outLogger.Println(string(stemcellVersionJson))

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

func (f FindStemcellVersion) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Prints the latest stemcell version from Pivnet using the stemcell type listed in the Kilnfile",
		ShortDescription: "prints the latest stemcell version from Pivnet using the stemcell type listed in the Kilnfile",
		Flags:            f.Options,
	}
}
