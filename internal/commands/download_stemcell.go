package commands

import (
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/go-pivnet/v2"
	"github.com/pivotal-cf/go-pivnet/v2/download"
	"github.com/pivotal-cf/go-pivnet/v2/logshim"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type DownloadStemcellInput struct {
	IaaS        string
	PivnetToken string
	ProductFile string //Kilnfile.lock as a first step, then eventually take metadata for 2.6 and earlier tiles
	DownloadDir string
	Version     string
	OS          string
	OutputDir   string
}

type StemcellClient struct {
	PivnetClient pivnet.Client
	Logger       *log.Logger
}

var iaasToPivNetRegex = map[string]string{
	"aws":          "AWS|aws",
	"vsphere":      "vSphere|vsphere",
	"azure":        "Azure|azure",
	"gcp":          "Google|GCP|google",
	"internetless": "Google|GCP|google",
}

type Stemcell struct {
	Options struct {
		flags.Standard
		IaaS        string `long:"iaas"        short:"i"  description:"specific iaas for the stemcell"`
		PivnetToken string `long:"pivnettoken" short:"pt" description:"token for network.pivotal.io"`
		ProductFile string `long:"productfile" short:"pf" description:"product file containing criteria location"`
		DownloadDir string `long:"downloaddir" short:"dd" description:"directory to download the stemcell to"`
		Version     string `long:"version"     short:"v"  description:"version of the stemcell"`
		OutputDir   string `long:"outputdir"   short:"o" description:"output directory?"`
	}
	FS     billy.Filesystem
	Logger *log.Logger

	ProductFiles productFiles
	Releases     releaseLister
	Eula         eulaAccepter
}

type (
	productFiles interface {
		ListForRelease(productSlug string, releaseID int) ([]pivnet.ProductFile, error)
		DownloadForRelease(
			location *download.FileInfo,
			productSlug string,
			releaseID int,
			productFileID int,
			progressWriter io.Writer,
		) error
	}
	releaseLister interface {
		List(productSlug string) ([]pivnet.Release, error)
	}
	eulaAccepter interface {
		Accept(productSlug string, releaseID int) error
	}
)

//counterfeiter:generate -o ./fakes/product_files.go --fake-name ProductFiles . productFiles
//counterfeiter:generate -o ./fakes/release_lister.go --fake-name ReleaseLister . releaseLister
//counterfeiter:generate -o ./fakes/eula_accepter.go --fake-name EulaAccepter . eulaAccepter

func NewDownloadStemcellCommand() (Stemcell, error) {
	s := Stemcell{}
	return s, nil
}

func (s *Stemcell) SetupPivnet(pivnetToken string) {
	//Logger Stuff
	stdoutLogger := log.New(os.Stdout, "", log.LstdFlags)
	stderrLogger := log.New(os.Stderr, "", log.LstdFlags)
	logger := logshim.NewLogShim(stdoutLogger, stderrLogger, true)
	s.Logger = stdoutLogger

	//Pivnet Client
	config := pivnet.ClientConfig{
		Host:              pivnet.DefaultHost,
		UserAgent:         "kiln",
		SkipSSLValidation: true,
	}

	//Hard-coding to remove later
	//pivnetToken := "wtxh-VGyz-Pdh8YgYPH6" //PARAM ME

	pivnetTokenService := pivnet.NewAccessTokenOrLegacyToken(pivnetToken, config.Host, config.SkipSSLValidation)
	client := pivnet.NewClient(pivnetTokenService, config, logger)
	s.ProductFiles = client.ProductFiles
	s.Releases = client.Releases
	s.Eula = client.EULA
}

func (s Stemcell) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "downloads a specific stemcell based on the specified criteria in a kilnfile.lock from the pivotal network",
		ShortDescription: "downloads a stemcell from pivotal network",
		Flags:            s.Options,
	}
}

func (s Stemcell) Execute(args []string) error {
	_, err := flags.LoadFlagsWithDefaults(&s.Options, args, s.FS.Stat)
	if err != nil {
		return err
	}

	//This will only work if the productfile is a kilnfile.lock
	_, kilnfileLock, err := s.Options.LoadKilnfiles(s.FS, nil)
	if err != nil {
		return fmt.Errorf("failed to load kilnfiles: %w", err)
	}

	if s.hasUnsetPivnetService() {
		s.SetupPivnet(s.Options.PivnetToken)
	}

	downloadStemcellInput := DownloadStemcellInput{
		OS:        kilnfileLock.Stemcell.OS,
		Version:   kilnfileLock.Stemcell.Version,
		IaaS:      "vsphere",                                     //PARAM ME
		OutputDir: "/Users/hallr/workspace/kiln/testDownloadDir", //PARAM ME
	}

	err = s.Download(downloadStemcellInput)

	if err != nil {
		log.Fatalln(err)
		return errors.New("download stemcell failed")
	}
	return nil
}

func (s Stemcell) hasUnsetPivnetService() bool {
	return s.ProductFiles == nil || s.Releases == nil || s.Eula == nil
}

func (s Stemcell) Download(input DownloadStemcellInput) error {
	stemcellProductSlug := getProductSlug(input.OS, input.IaaS)

	releaseID, err := s.getReleaseIDByStemcellVersion(stemcellProductSlug, input.Version)
	if err != nil {
		return err
	}

	panic("BLA BLA BLA")

	err = s.Eula.Accept(stemcellProductSlug, releaseID)
	if err != nil {
		return err
	}

	stemcellFile, err := s.getProductFileByRegex(stemcellProductSlug, releaseID, iaasToPivNetRegex[input.IaaS])
	if err != nil {
		return err
	}
	return s.downloadStemcell(input.DownloadDir, stemcellFile, stemcellProductSlug, releaseID)
}

func (s Stemcell) downloadStemcell(downloadDir string, productFile pivnet.ProductFile, productSlug string, releaseID int) error {
	destFilepath := filepath.Join(downloadDir, path.Base(productFile.AWSObjectKey))
	destFile, err := os.Create(destFilepath)
	if err != nil {
		return fmt.Errorf("Failed to create output file for the stemcell: %s", err)
	}
	s.Logger.Printf("Downloading stemcell file '%s' to '%s'...\n", productFile.Name, destFilepath)

	tmpLocation, err := download.NewFileInfo(destFile)
	return s.ProductFiles.DownloadForRelease(tmpLocation, productSlug, releaseID, productFile.ID, os.Stderr)
}

func getProductSlug(os, iaas string) string {
	var stemcellProductSlug string
	if strings.Contains(os, "windows") {
		if strings.Contains(iaas, "vsphere") {
			stemcellProductSlug = "stemcells-windows-server-internal"
		} else {
			stemcellProductSlug = "stemcells-windows-server"
		}
	} else if strings.Contains(os, "xenial") {
		stemcellProductSlug = "stemcells-ubuntu-xenial"
	} else {
		stemcellProductSlug = "stemcells"
	}
	return stemcellProductSlug
}

func (s Stemcell) getReleaseIDByStemcellVersion(stemcellProductSlug string, stemcellVersion string) (int, error) {
	releases, err := s.Releases.List(stemcellProductSlug)

	if err != nil {
		return 0, err
	}
	var releaseID int
	for _, release := range releases {
		if release.Version == stemcellVersion {
			releaseID = release.ID
			break
		}
	}

	if releaseID == 0 {
		// If no release was found, try to find the latest release for the major version
		major := strings.SplitN(stemcellVersion, ".", 2)[0]

		fmt.Printf("Could not find release for stemcell version %s\n. Trying to find the latest available version for its major.\n", stemcellVersion)
		for _, release := range releases {
			if strings.HasPrefix(release.Version, major) {
				releaseID = release.ID
				break
			}
		}
	}

	if releaseID == 0 {
		return 0, fmt.Errorf("Failed to find stemcell with release version: %s", stemcellVersion)
	}

	return releaseID, nil
}

func (s Stemcell) getProductFileByRegex(productSlug string, releaseID int, nameRegex string) (pivnet.ProductFile, error) {
	productFiles, err := s.ProductFiles.ListForRelease(productSlug, releaseID)
	if err != nil {
		return pivnet.ProductFile{}, err
	}

	nameRe := regexp.MustCompile(nameRegex)
	var matchedFile pivnet.ProductFile

	for _, productFile := range productFiles {
		if nameRe.MatchString(productFile.Name) {
			matchedFile = productFile
			break
		}
	}

	if matchedFile.Name == "" {
		return pivnet.ProductFile{}, fmt.Errorf("Failed to find stemcell file matching regex '%s'", nameRegex)
	}
	return matchedFile, nil
}
