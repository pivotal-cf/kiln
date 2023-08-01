package cargo

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"

	"github.com/cloudfoundry/bosh-cli/v7/director"

	"github.com/pivotal-cf/kiln/pkg/proofing"
	"github.com/pivotal-cf/kiln/pkg/tile"
)

func ReadBOSHReleaseFromFile(tilePath, releaseName, releaseVersion string, releaseTarball io.Writer) (proofing.Release, error) {
	f, err := os.Open(tilePath)
	if err != nil {
		return proofing.Release{}, err
	}
	defer closeAndIgnoreError(f)
	fi, err := f.Stat()
	if err != nil {
		return proofing.Release{}, err
	}
	return ReadBOSHReleaseFromZip(f, fi.Size(), releaseName, releaseVersion, releaseTarball)
}

func ReadBOSHReleaseFromZip(ra io.ReaderAt, zipFileSize int64, releaseName, releaseVersion string, releaseTarball io.Writer) (proofing.Release, error) {
	zr, err := zip.NewReader(ra, zipFileSize)
	if err != nil {
		return proofing.Release{}, fmt.Errorf("failed to do open metadata zip reader: %w", err)
	}
	return ReadBOSHReleaseFromFS(zr, releaseName, releaseVersion, releaseTarball)
}

func ReadBOSHReleaseFromFS(dir fs.FS, releaseName, releaseVersion string, releaseTarball io.Writer) (proofing.Release, error) {
	metadataBuf, err := tile.ReadMetadataFromFS(dir)
	if err != nil {
		return proofing.Release{}, err
	}

	var metadata struct {
		Releases []proofing.Release `yaml:"releases"`
	}
	err = yaml.Unmarshal(metadataBuf, &metadata)
	if err != nil {
		return proofing.Release{}, err
	}

	releaseIndex := slices.IndexFunc(metadata.Releases, func(release proofing.Release) bool {
		return release.Name == releaseName && release.Version == releaseVersion
	})
	if releaseIndex == -1 {
		return proofing.Release{}, fmt.Errorf("release not found with %s/%s", releaseName, releaseVersion)
	}
	release := metadata.Releases[releaseIndex]

	f, err := dir.Open(path.Join("releases", release.File))
	if err != nil {
		return proofing.Release{}, err
	}
	defer closeAndIgnoreError(f)

	_, err = io.Copy(releaseTarball, f)
	if err != nil {
		return proofing.Release{}, fmt.Errorf("failed to copy release tarball: %w", err)
	}

	return release, nil
}

type BOSHReleasePackage struct {
	Name         string   `yaml:"name"`
	Version      string   `yaml:"version"`
	Fingerprint  string   `yaml:"fingerprint"`
	SHA1         string   `yaml:"sha1"`
	Dependencies []string `yaml:"dependencies"`
}

type CompiledBOSHReleasePackage struct {
	Name         string   `yaml:"name"`
	Version      string   `yaml:"version"`
	Fingerprint  string   `yaml:"fingerprint"`
	SHA1         string   `yaml:"sha1"`
	Dependencies []string `yaml:"dependencies"`

	Stemcell string `yaml:"stemcell"`
}

type BOSHReleaseManifest struct {
	Name               string `yaml:"name,omitempty"`
	Version            string `yaml:"version,omitempty"`
	CommitHash         string `yaml:"commit_hash,omitempty"`
	UncommittedChanges bool   `yaml:"uncommitted_changes"`

	CompiledPackages []CompiledBOSHReleasePackage `yaml:"compiled_packages"`
	Packages         []BOSHReleasePackage         `yaml:"packages"`
}

func (mf BOSHReleaseManifest) Stemcell() (string, string, bool) {
	if len(mf.CompiledPackages) == 0 {
		return "", "", false
	}
	return strings.Cut(mf.CompiledPackages[0].Stemcell, "/")
}

type BOSHReleaseTarball struct {
	Manifest BOSHReleaseManifest

	SHA1     string
	FilePath string
}

func ReadBOSHReleaseTarball(tarballPath string) (BOSHReleaseTarball, error) {
	file, err := os.Open(tarballPath)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	defer closeAndIgnoreError(file)
	m, err := ReadProductTemplatePartFromBOSHReleaseTarball(file)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	s, err := calculateChecksum(sha1.New())(file)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	return BOSHReleaseTarball{
		Manifest: m,
		SHA1:     s,
		FilePath: tarballPath,
	}, nil
}

func ReadBOSHReleaseManifestsFromTarballs(dir fs.FS, tarballPaths ...string) ([]BOSHReleaseTarball, error) {
	results := make([]BOSHReleaseTarball, 0, len(tarballPaths))
	for _, tarballPath := range tarballPaths {
		mf, err := openAndProcessFile(dir, tarballPath, ReadProductTemplatePartFromBOSHReleaseTarball)
		if err != nil {
			return nil, err
		}
		sha1Checksum, err := openAndProcessFile(dir, tarballPath, calculateChecksum(sha1.New()))
		if err != nil {
			return nil, err
		}

		results = append(results, BOSHReleaseTarball{
			Manifest: mf,
			SHA1:     sha1Checksum,
			FilePath: tarballPath,
		})
	}
	return slices.Clip(results), nil
}

func openAndProcessFile[T any](dir fs.FS, fileName string, process func(io.Reader) (T, error)) (T, error) {
	file, err := dir.Open(fileName)
	if err != nil {
		var zero T
		return zero, err
	}
	defer closeAndIgnoreError(file)
	return process(file)
}

func ReadProductTemplatePartFromBOSHReleaseTarball(r io.Reader) (BOSHReleaseManifest, error) {
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return BOSHReleaseManifest{}, err
	}
	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return BOSHReleaseManifest{}, err
			}
			break
		}
		if path.Base(header.Name) != "release.MF" {
			continue
		}
		releaseMFBuffer, err := io.ReadAll(tarReader)
		if err != nil {
			return BOSHReleaseManifest{}, err
		}

		var releaseMF BOSHReleaseManifest

		if err := yaml.Unmarshal(releaseMFBuffer, &releaseMF); err != nil {
			return BOSHReleaseManifest{}, err
		}

		return releaseMF, nil
	}
	return BOSHReleaseManifest{}, fmt.Errorf("failed to find release.MF in tarball")
}

func calculateChecksum(h hash.Hash) func(r io.Reader) (string, error) {
	return func(r io.Reader) (string, error) {
		_, err := io.Copy(h, r)
		if err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
}

func CompileBOSHReleaseTarballs(_ context.Context, logger *log.Logger, boshDirector director.Director, sc Stemcell, uploadTries int, tarballs ...BOSHReleaseTarball) ([]BOSHReleaseTarball, error) {
	if uploadTries == 0 {
		uploadTries = 5
	}
	err := ensureBOSHDirectorHasStemcell(sc, boshDirector)
	if err != nil {
		return nil, err
	}

	var releasesToCompile []BOSHReleaseTarball
	for _, tarball := range tarballs {
		if len(tarball.Manifest.CompiledPackages) > 0 {
			return nil, fmt.Errorf("%s/%s (%s) has compiled packages", tarball.Manifest.Name, tarball.Manifest.Version, tarball.FilePath)
		}
		for try := 1; try <= uploadTries; try++ {
			logger.Printf("Uploading BOSH Release %s/%s [try %d]", tarball.Manifest.Name, tarball.Manifest.Version, try)
			err = uploadBOSHReleaseTarballToDirector(boshDirector, sc, tarball)
			if err != nil {
				log.Printf("try %d of %d failed with error: %s", try, uploadTries, err)
				continue
			}
			break
		}
		if err != nil {
			return nil, err
		}
		releasesToCompile = append(releasesToCompile, tarball)
	}

	manifest, deploymentName, err := deploymentManifest(releasesToCompile, sc)
	if err != nil {
		return nil, err
	}

	logger.Printf("Deployment Manifest\n%s\n\n", string(manifest))

	deployment, err := boshDirector.FindDeployment(deploymentName)
	if err != nil {
		return nil, err
	}

	err = deployment.Update(manifest, director.UpdateOpts{})
	if err != nil {
		return nil, err
	}

	result := make([]BOSHReleaseTarball, 0, len(releasesToCompile))
	for _, tarball := range releasesToCompile {
		logger.Printf("Exporting and Downloading BOSH Release %s/%s", tarball.Manifest.Name, tarball.Manifest.Version)
		boshReleaseTarball, err := exportAndDownloadBOSHRelease(boshDirector, deployment, tarball, sc)
		if err != nil {
			return nil, err
		}
		logger.Printf("finished compiling and exporting %s/%s with stemcell %s/%s", tarball.Manifest.Name, tarball.Manifest.Version, sc.OS, sc.Version)
		result = append(result, boshReleaseTarball)
	}
	return result, nil
}

func exportAndDownloadBOSHRelease(boshDirector director.Director, deployment director.Deployment, source BOSHReleaseTarball, sc Stemcell) (BOSHReleaseTarball, error) {
	releaseMF := source.Manifest
	exportFileName := fmt.Sprintf("%s-%s-%s-%s.tgz", releaseMF.Name, releaseMF.Version, sc.OS, sc.Version)
	exportFilePath := filepath.Join(filepath.Dir(source.FilePath), exportFileName)

	releaseSlug := director.NewReleaseSlug(releaseMF.Name, releaseMF.Version)
	stemcellSlug := director.NewOSVersionSlug(sc.OS, sc.Version)
	exportResult, err := deployment.ExportRelease(releaseSlug, stemcellSlug, nil)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	f, err := os.Create(exportFilePath)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	defer closeAndIgnoreError(f)

	var (
		downloadCheck    = sha1.New()
		expectedChecksum = exportResult.SHA1
	)
	if strings.HasPrefix(exportResult.SHA1, "sha256:") {
		downloadCheck = sha256.New()
		expectedChecksum = strings.TrimPrefix(exportResult.SHA1, "sha256:")
	}
	if strings.HasPrefix(exportResult.SHA1, "sha512:") {
		downloadCheck = sha512.New()
		expectedChecksum = strings.TrimPrefix(exportResult.SHA1, "sha512:")
	}
	mr := io.MultiWriter(f, downloadCheck)
	err = boshDirector.DownloadResourceUnchecked(exportResult.BlobstoreID, mr)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	tarballCheckSum := hex.EncodeToString(downloadCheck.Sum(nil))
	if tarballCheckSum != expectedChecksum {
		return BOSHReleaseTarball{}, fmt.Errorf("download checksum does not match expected checksum: %s != %s", tarballCheckSum, exportResult.SHA1)
	}
	return BOSHReleaseTarball{
		Manifest: releaseMF,
		SHA1:     tarballCheckSum,
		FilePath: exportFilePath,
	}, nil
}

func uploadBOSHReleaseTarballToDirector(boshDirector director.Director, sc Stemcell, tarball BOSHReleaseTarball) error {
	release, findReleaseErr := boshDirector.FindRelease(director.NewReleaseSlug(tarball.Manifest.Name, tarball.Manifest.Version))
	if findReleaseErr == nil {
		exists, err := release.Exists()
		if err != nil {
			return errors.Join(findReleaseErr, err)
		}
		if exists {
			directorPackages, err := release.Packages()
			if err != nil {
				return err
			}
			for _, pkg := range tarball.Manifest.Packages {
				if slices.IndexFunc(directorPackages, func(p director.Package) bool {
					return p.Name == pkg.Name && pkg.SHA1 == p.SHA1 && p.Fingerprint == pkg.Fingerprint && slices.IndexFunc(p.CompiledPackages, func(compiledPackage director.CompiledPackage) bool {
						return compiledPackage.Stemcell.OS() == sc.OS && compiledPackage.Stemcell.OS() == sc.Version
					}) >= 0
				}) < 0 {
					break
				}
			}
		}
	}
	f, err := os.Open(tarball.FilePath)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(f)
	if err := boshDirector.UploadReleaseFile(f, false, false); err != nil {
		return err
	}
	release, err = boshDirector.FindRelease(director.NewReleaseSlug(tarball.Manifest.Name, tarball.Manifest.Version))
	if err != nil {
		return err
	}
	exists, err := release.Exists()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("failed to upload release %s/%s", tarball.Manifest.Name, tarball.Manifest.Version)
	}
	return nil
}

func ensureBOSHDirectorHasStemcell(sc Stemcell, boshDirector director.Director) error {
	_, err := boshDirector.FindStemcell(director.NewStemcellSlug(sc.OS, sc.Version))
	return err
}

func deploymentManifest(releases []BOSHReleaseTarball, stemcell Stemcell) ([]byte, string, error) {
	type deploymentManifestRelease = struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
		SHA1    string `yaml:"sha1"`
	}
	type deploymentManifestStemcell = struct {
		Alias   string `yaml:"alias"`
		OS      string `yaml:"os"`
		Version string `yaml:"version"`
	}
	type deploymentManifestUpdate = struct {
		Canaries        int    `yaml:"canaries"`
		MaxInFlight     int    `yaml:"max_in_flight"`
		CanaryWatchTime string `yaml:"canary_watch_time"`
		UpdateWatchTime string `yaml:"update_watch_time"`
	}
	var manifest struct {
		Name           string                       `yaml:"name"`
		Releases       []deploymentManifestRelease  `yaml:"releases"`
		Stemcells      []deploymentManifestStemcell `yaml:"stemcells"`
		Update         deploymentManifestUpdate     `yaml:"update"`
		InstanceGroups []interface{}                `yaml:"instance_groups"`
	}

	manifest.Update = deploymentManifestUpdate{
		Canaries:        1,
		MaxInFlight:     1,
		CanaryWatchTime: "1000-1001",
		UpdateWatchTime: "1000-1001",
	}

	if stemcell.Alias == "" {
		stemcell.Alias = "default"
	}
	manifest.Stemcells = []deploymentManifestStemcell{
		{
			Alias:   stemcell.Alias,
			OS:      stemcell.OS,
			Version: stemcell.Version,
		},
	}

	shaSuffix := sha1.New()

	manifest.Releases = make([]deploymentManifestRelease, 0, len(releases))
	for _, r := range releases {
		_, _ = io.WriteString(shaSuffix, r.Manifest.Name)
		_, _ = io.WriteString(shaSuffix, r.Manifest.CommitHash)
		manifest.Releases = append(manifest.Releases, deploymentManifestRelease{
			Name:    r.Manifest.Name,
			Version: r.Manifest.Version,
			SHA1:    r.SHA1,
		})
	}
	checkSum := shaSuffix.Sum(nil)
	manifest.Name = fmt.Sprintf("kiln-%s-%d", hex.EncodeToString(checkSum), time.Now().Unix())
	m, err := yaml.Marshal(manifest)
	return m, manifest.Name, err
}
