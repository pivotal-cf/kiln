package commands

import (
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/om/api"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/om"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

//counterfeiter:generate -o ./fakes/ops_manager_release_cache_source.go --fake-name OpsManagerReleaseCacheSource . OpsManagerReleaseCacheSource
//counterfeiter:generate -o ./fakes/release_storage.go --fake-name ReleaseStorage . ReleaseStorage

type (
	OpsManagerReleaseCacheSource interface {
		om.GetBoshEnvironmentAndSecurityRootCACertificateProvider
		GetStagedProductManifest(guid string) (string, error)
		GetStagedProductByName(productName string) (api.StagedProductsFindOutput, error)
	}

	ReleaseStorage interface {
		component.ReleaseSource
		UploadRelease(spec cargo.BOSHReleaseTarballSpecification, file io.Reader) (cargo.BOSHReleaseTarballLock, error)
	}
)

type CacheCompiledReleases struct {
	Options struct {
		flags.Standard
		om.ClientConfiguration

		UploadTargetID string `           long:"upload-target-id"   required:"true"    description:"the ID of the release source where the built release will be uploaded"`
		ReleasesDir    string `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
		Name           string `short:"n"  long:"name"               default:"cf"       description:"name of the tile"` // TODO: parse from base.yml
	}

	Logger *log.Logger
	FS     billy.Filesystem

	ReleaseSourceAndCache func(kilnfile cargo.Kilnfile, targetID string) (ReleaseStorage, error)
	OpsManager            func(om.ClientConfiguration) (OpsManagerReleaseCacheSource, error)
	Director              func(om.ClientConfiguration, om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (boshdir.Director, error)
}

func NewCacheCompiledReleases() *CacheCompiledReleases {
	cmd := &CacheCompiledReleases{
		FS:     osfs.New(""),
		Logger: log.Default(),
	}
	cmd.ReleaseSourceAndCache = func(kilnfile cargo.Kilnfile, targetID string) (ReleaseStorage, error) {
		releaseSource, err := component.NewReleaseSourceRepo(kilnfile, cmd.Logger).FindByID(targetID)
		if err != nil {
			return nil, err
		}
		releaseCache, ok := releaseSource.(ReleaseStorage)
		if !ok {
			return nil, fmt.Errorf("unsupported release source type %T: it does not implement the required methods", releaseSource)
		}
		return releaseCache, nil
	}
	cmd.OpsManager = func(conf om.ClientConfiguration) (OpsManagerReleaseCacheSource, error) {
		return conf.API()
	}
	cmd.Director = om.BoshDirector
	return cmd
}

func (cmd *CacheCompiledReleases) WithLogger(logger *log.Logger) *CacheCompiledReleases {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	cmd.Logger = logger
	return cmd
}

func (cmd *CacheCompiledReleases) Execute(args []string) error {
	_, err := flags.LoadWithDefaults(&cmd.Options, args, cmd.FS.Stat)
	if err != nil {
		return err
	}

	kilnfile, lock, err := cmd.Options.LoadKilnfiles(cmd.FS, nil)
	if err != nil {
		return fmt.Errorf("failed to load kilnfiles: %w", err)
	}

	omAPI, deploymentName, stagedStemcellOS, stagedStemcellVersion, err := cmd.fetchProductDeploymentData()
	if err != nil {
		return err
	}

	if stagedStemcellOS != lock.Stemcell.OS || stagedStemcellVersion != lock.Stemcell.Version {
		return fmt.Errorf(
			"staged stemcell (%s %s) and lock stemcell (%s %s) do not match",
			stagedStemcellOS, stagedStemcellVersion,
			lock.Stemcell.OS, lock.Stemcell.Version,
		)
	}

	releaseStore, err := cmd.ReleaseSourceAndCache(kilnfile, cmd.Options.UploadTargetID)
	if err != nil {
		return fmt.Errorf("failed to configure release source: %w", err)
	}

	var (
		releasesToExport         []cargo.BOSHReleaseTarballLock
		releasesUpdatedFromCache = false
	)
	for _, rel := range lock.Releases {
		remote, err := releaseStore.GetMatchedRelease(cargo.BOSHReleaseTarballSpecification{
			Name:            rel.Name,
			Version:         rel.Version,
			StemcellOS:      lock.Stemcell.OS,
			StemcellVersion: lock.Stemcell.Version,
		})
		if err != nil {
			if !component.IsErrNotFound(err) {
				return fmt.Errorf("failed check for matched release: %w", err)
			}
			releasesToExport = append(releasesToExport, cargo.BOSHReleaseTarballLock{
				Name:            rel.Name,
				Version:         rel.Version,
				StemcellOS:      lock.Stemcell.OS,
				StemcellVersion: lock.Stemcell.Version,
			})
			continue
		}

		cmd.Logger.Printf("found %s/%s in %s\n", rel.Name, rel.Version, remote.RemoteSource)

		sum, err := cmd.downloadAndComputeSHA(releaseStore, remote)
		if err != nil {
			cmd.Logger.Printf("unable to get hash sum for %s", remote.ReleaseSlug())
			continue
		}
		remote.SHA1 = sum

		releasesUpdatedFromCache = true
		err = updateLock(lock, remote, cmd.Options.UploadTargetID)
		if err != nil {
			return fmt.Errorf("failed to update lock file: %w", err)
		}
	}

	switch len(releasesToExport) {
	case 0:
		cmd.Logger.Print("cache already contains releases matching constraint\n")
		if releasesUpdatedFromCache {
			err = cmd.Options.Standard.SaveKilnfileLock(cmd.FS, lock)
			if err != nil {
				return err
			}

			cmd.Logger.Printf("DON'T FORGET TO MAKE A COMMIT AND PR\n")

			return nil
		}
	case 1:
		cmd.Logger.Printf("1 release needs to be exported and cached\n")
	default:
		cmd.Logger.Printf("%d releases need to be exported and cached\n", len(releasesToExport))
	}

	for _, rel := range releasesToExport {
		cmd.Logger.Printf("\t%s compiled with %s not found in cache\n", rel.ReleaseSlug(), rel.StemcellSlug())
	}

	bosh, err := cmd.Director(cmd.Options.ClientConfiguration, omAPI)
	if err != nil {
		return err
	}

	deployment, err := bosh.FindDeployment(deploymentName)
	if err != nil {
		return err
	}

	cmd.Logger.Printf("exporting from bosh deployment %s\n", deploymentName)

	err = cmd.FS.MkdirAll(cmd.Options.ReleasesDir, 0o777)
	if err != nil {
		return fmt.Errorf("failed to create release directory: %w", err)
	}

	for _, rel := range releasesToExport {
		releaseSlug := rel.ReleaseSlug()
		stemcellSlug := boshdir.NewOSVersionSlug(stagedStemcellOS, stagedStemcellVersion)

		hasRelease, err := hasRequiredCompiledPackages(bosh, rel.ReleaseSlug(), stemcellSlug)
		if err != nil {
			if !errors.Is(err, errNoPackages) {
				return fmt.Errorf("failed to find release %s: %w", rel.ReleaseSlug(), err)
			}
			cmd.Logger.Printf("%s does not have any packages\n", rel)
		}
		if !hasRelease {
			return fmt.Errorf("%[1]s compiled with %[2]s is not found on bosh director (it might have been uploaded as a compiled release and the director can't recompile it for the compilation target %[2]s)", releaseSlug, stemcellSlug)
		}

		newRemote, err := cmd.cacheRelease(bosh, releaseStore, deployment, releaseSlug, stemcellSlug)
		if err != nil {
			cmd.Logger.Printf("\tfailed to cache release %s for %s: %s\n", releaseSlug, stemcellSlug, err)
			continue
		}

		err = updateLock(lock, newRemote, cmd.Options.UploadTargetID)
		if err != nil {
			return fmt.Errorf("failed to lock release %s: %w", rel.Name, err)
		}
	}

	err = cmd.Options.Standard.SaveKilnfileLock(cmd.FS, lock)
	if err != nil {
		return err
	}

	cmd.Logger.Printf("DON'T FORGET TO MAKE A COMMIT AND PR\n")

	return nil
}

var errNoPackages = errors.New("release has no packages")

// hasRequiredCompiledPackages implementation is copied from the boshdir.DirectorImpl HasRelease method. It adds the check
// for the length of the packages slice to allow for BOSH releases that do not have any packages. One example of a BOSH
// release without packages is https://github.com/cloudfoundry/bosh-dns-aliases-release.
func hasRequiredCompiledPackages(d boshdir.Director, releaseSlug boshdir.ReleaseSlug, stemcell boshdir.OSVersionSlug) (bool, error) {
	release, err := d.FindRelease(releaseSlug)
	if err != nil {
		return false, err
	}

	pkgs, err := release.Packages()
	if err != nil {
		return false, err
	}

	if len(pkgs) == 0 {
		return true, errNoPackages
	}

	for _, pkg := range pkgs {
		for _, compiledPkg := range pkg.CompiledPackages {
			if compiledPkg.Stemcell == stemcell {
				return true, nil
			}
		}
	}

	return false, nil
}

func (cmd *CacheCompiledReleases) fetchProductDeploymentData() (_ OpsManagerReleaseCacheSource, deploymentName, stemcellOS, stemcellVersion string, _ error) {
	omAPI, err := cmd.OpsManager(cmd.Options.ClientConfiguration)
	if err != nil {
		return nil, "", "", "", err
	}

	stagedProduct, err := omAPI.GetStagedProductByName(cmd.Options.Name)
	if err != nil {
		return nil, "", "", "", err
	}

	stagedManifest, err := omAPI.GetStagedProductManifest(stagedProduct.Product.GUID)
	if err != nil {
		return nil, "", "", "", err
	}

	var manifest struct {
		Name      string `yaml:"name"`
		Stemcells []struct {
			OS      string `yaml:"os"`
			Version string `yaml:"version"`
		} `yaml:"stemcells"`
	}

	if err := yaml.Unmarshal([]byte(stagedManifest), &manifest); err != nil {
		return nil, "", "", "", err
	}

	if len(manifest.Stemcells) == 0 {
		return nil, "", "", "", errors.New("manifest stemcell not set")
	}
	stagedStemcell := manifest.Stemcells[0]

	return omAPI, manifest.Name, stagedStemcell.OS, stagedStemcell.Version, nil
}

func (cmd *CacheCompiledReleases) cacheRelease(bosh boshdir.Director, rc ReleaseStorage, deployment boshdir.Deployment, releaseSlug boshdir.ReleaseSlug, stemcellSlug boshdir.OSVersionSlug) (cargo.BOSHReleaseTarballLock, error) {
	cmd.Logger.Printf("\texporting %s\n", releaseSlug)
	result, err := deployment.ExportRelease(releaseSlug, stemcellSlug, nil)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	cmd.Logger.Printf("\tdownloading %s\n", releaseSlug)
	releaseFilePath, _, sha1sum, err := cmd.saveReleaseLocally(bosh, cmd.Options.ReleasesDir, releaseSlug, stemcellSlug, result)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	cmd.Logger.Printf("\tuploading %s\n", releaseSlug)
	remoteRelease, err := cmd.uploadLocalRelease(cargo.BOSHReleaseTarballSpecification{
		Name:            releaseSlug.Name(),
		Version:         releaseSlug.Version(),
		StemcellOS:      stemcellSlug.OS(),
		StemcellVersion: stemcellSlug.Version(),
	}, releaseFilePath, rc)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	remoteRelease.SHA1 = sha1sum

	return remoteRelease, nil
}

func updateLock(lock cargo.KilnfileLock, release cargo.BOSHReleaseTarballLock, targetID string) error {
	for index, releaseLock := range lock.Releases {
		if release.Name != releaseLock.Name {
			continue
		}

		checksum := release.SHA1
		if releaseLock.RemoteSource == targetID {
			checksum = releaseLock.SHA1
		}

		lock.Releases[index] = cargo.BOSHReleaseTarballLock{
			Name:         release.Name,
			Version:      release.Version,
			RemoteSource: release.RemoteSource,
			RemotePath:   release.RemotePath,
			SHA1:         checksum,
		}
		return nil
	}
	return fmt.Errorf("existing release not found in Kilnfile.lock")
}

func (cmd *CacheCompiledReleases) uploadLocalRelease(spec cargo.BOSHReleaseTarballSpecification, fp string, uploader ReleaseStorage) (cargo.BOSHReleaseTarballLock, error) {
	f, err := cmd.FS.Open(fp)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}
	defer closeAndIgnoreError(f)
	return uploader.UploadRelease(spec, f)
}

func (cmd *CacheCompiledReleases) saveReleaseLocally(director boshdir.Director, relDir string, releaseSlug boshdir.ReleaseSlug, stemcellSlug boshdir.OSVersionSlug, res boshdir.ExportReleaseResult) (string, string, string, error) {
	fileName := fmt.Sprintf("%s-%s-%s-%s.tgz", releaseSlug.Name(), releaseSlug.Version(), stemcellSlug.OS(), stemcellSlug.Version())
	filePath := filepath.Join(relDir, fileName)

	f, err := cmd.FS.Create(filePath)
	if err != nil {
		return "", "", "", err
	}
	defer closeAndIgnoreError(f)

	sha256sum := sha256.New()
	sha1sum := sha1.New()

	w := io.MultiWriter(f, sha256sum, sha1sum)

	err = director.DownloadResourceUnchecked(res.BlobstoreID, w)
	if err != nil {
		_ = os.Remove(filePath)
		return "", "", "", err
	}

	sha256sumString := fmt.Sprintf("%x", sha256sum.Sum(nil))
	sha1sumString := fmt.Sprintf("%x", sha1sum.Sum(nil))

	if sum := fmt.Sprintf("sha256:%s", sha256sumString); sum != res.SHA1 {
		return "", "", "", fmt.Errorf("checksums do not match got %q but expected %q", sum, res.SHA1)
	}

	return filePath, sha256sumString, sha1sumString, nil
}

func (cmd *CacheCompiledReleases) downloadAndComputeSHA(cache component.ReleaseSource, remote cargo.BOSHReleaseTarballLock) (string, error) {
	if remote.SHA1 != "" {
		return remote.SHA1, nil
	}

	tmpdir, err := os.MkdirTemp("/tmp", "kiln")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		err = os.RemoveAll(tmpdir)
		if err != nil {
			cmd.Logger.Printf("unable to delete '%s': %s", tmpdir, err)
		}
	}()

	comp, err := cache.DownloadRelease(tmpdir, remote)
	if err != nil {
		return "", fmt.Errorf("failed to download release: %s", err)
	}

	return comp.Lock.SHA1, nil
}

func (cmd *CacheCompiledReleases) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Downloads compiled bosh releases from an Tanzu Ops Manager bosh director and then uploads them to a bucket",
		ShortDescription: "Cache compiled releases",
		Flags:            cmd.Options,
	}
}
