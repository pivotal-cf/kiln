package fetch_compiled_releases

import (
	"crypto/sha256"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	"github.com/pivotal-cf/kiln/internal/fetcher"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/release"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/om"
)

type Command struct {
	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile"           default:"Kilnfile" description:"path to Kilnfile"`
		VariablesFiles []string `short:"vf" long:"variables-file"                        description:"path to variables file"`
		Variables      []string `short:"vr" long:"variable"                              description:"variable in key=value format"`
		ReleasesDir    string   `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
		Name           string   `short:"n"  long:"name"               default:"cf"       description:"name of the tile"` // TODO: parse from base.yml

		om.ClientConfiguration
	}
}

var _ jhanda.Command = (*Command)(nil)

func (cmd Command) Execute(args []string) error {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return fmt.Errorf("failed to parse comand arguments: %w", err)
	}

	fs := osfs.New("")
	kilnfile, lock, err := cargo.KilnfileLoader{}.LoadKilnfiles(fs,
		cmd.Options.Kilnfile,
		cmd.Options.VariablesFiles,
		cmd.Options.Variables,
	)
	if err != nil {
		return fmt.Errorf("failed to load kilnfiles: %w", err)
	}

	releaseSourceProvider := fetcher.NewReleaseSourceRepo(kilnfile, log.New(ioutil.Discard, "", 0))
	releaseSource := releaseSourceProvider.MultiReleaseSource(false)

	nonCompiledReleases, err := findBuiltReleases(releaseSource, lock)
	if err != nil {
		return err
	}

	fmt.Printf("%d releases in the lockfile are not compiled\n", len(nonCompiledReleases))
	for _, rel := range nonCompiledReleases {
		fmt.Printf("\t%s %s\n", rel.Name, rel.Version)
	}

	if len(nonCompiledReleases) == 0 {
		return nil
	}

	omAPI, err := cmd.Options.ClientConfiguration.API()
	if err != nil {
		return err
	}

	stagedProduct, err := omAPI.GetStagedProductByName(cmd.Options.Name)
	if err != nil {
		return err
	}

	stagedManifest, err := omAPI.GetStagedProductManifest(stagedProduct.Product.GUID)
	if err != nil {
		return err
	}

	var manifest struct {
		Name      string `yaml:"name"`
		Stemcells []struct {
			Alias   string `yaml:"alias"`
			OS      string `yaml:"os"`
			Version string `yaml:"version"`
		} `yaml:"stemcells"`
	}

	if err := yaml.Unmarshal([]byte(stagedManifest), &manifest); err != nil {
		return err
	}

	stagedStemcell := manifest.Stemcells[0]

	bosh, err := om.BoshDirector(cmd.Options.ClientConfiguration, omAPI)
	if err != nil {
		return err
	}

	osVersionSlug := boshdir.NewOSVersionSlug(stagedStemcell.OS, stagedStemcell.Version)

	deployment, err := bosh.FindDeployment(manifest.Name)
	if err != nil {
		return err
	}

	fmt.Printf("exporting from bosh deployment %s\n", manifest.Name)

	var exportedReleases []exportedRelease
	for i, rel := range nonCompiledReleases {
		fmt.Printf("\texporting %s %s\n", rel.Name, rel.Version)

		result, err := deployment.ExportRelease(boshdir.NewReleaseSlug(rel.Name, rel.Version), osVersionSlug, nil)
		if err != nil {
			return err
		}
		exportedReleases = append(exportedReleases, exportedRelease{
			Export:  result,
			Release: nonCompiledReleases[i],
		})
	}

	_ = os.MkdirAll(cmd.Options.ReleasesDir, 0777)

	var releaseFilePaths []string
	for _, exp := range exportedReleases {
		fmt.Printf("\tdownloading %s %s\n", exp.Release.Name, exp.Release.Version)

		releaseFilePath, err := saveRelease(bosh, cmd.Options.ReleasesDir, exp, stagedStemcell.OS, stagedStemcell.Version)
		if err != nil {
			return err
		}

		releaseFilePaths = append(releaseFilePaths, releaseFilePath)
	}

	return nil
}

type exportedRelease struct {
	Release release.Remote
	Export  boshdir.ExportReleaseResult
}

func saveRelease(director boshdir.Director, relDir string, exp exportedRelease, stemcellOS, stemcellVersion string) (string, error) {
	fileName := fmt.Sprintf("%s-%s-%s-%s.tgz", exp.Release.Name, exp.Release.Version, stemcellOS, stemcellVersion)
	filePath := filepath.Join(relDir, fileName)

	f, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()

	checkSum := sha256.New()

	w := io.MultiWriter(f, checkSum)

	err = director.DownloadResourceUnchecked(exp.Export.BlobstoreID, w)
	if err != nil {
		_ = os.Remove(filePath)
		return "", err
	}

	if sum := fmt.Sprintf("sha256:%x", checkSum.Sum(nil)); sum != exp.Export.SHA1 {
		return "", fmt.Errorf("checksums do not match got %q but expected %q", sum, exp.Export.SHA1)
	}

	return filePath, nil
}

func (cmd Command) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Validate checks for common Kilnfile and Kilnfile.lock mistakes",
		ShortDescription: "validate Kilnfile and Kilnfile.lock",
		Flags:            cmd.Options,
	}
}

func findBuiltReleases(allReleaseSources fetcher.MultiReleaseSource, kilnfileLock cargo.KilnfileLock) ([]release.Remote, error) {
	var builtReleases []release.Remote
	for _, lock := range kilnfileLock.Releases {
		src, err := allReleaseSources.FindByID(lock.RemoteSource)
		if err != nil {
			return nil, err
		}
		if src.Publishable() {
			continue
		}
		builtReleases = append(builtReleases, release.Remote{
			ID:         release.ID{Name: lock.Name, Version: lock.Version},
			SourceID:   lock.RemoteSource,
			RemotePath: lock.RemotePath,
		})
	}
	return builtReleases, nil
}
