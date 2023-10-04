package cargo

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

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

func OpenBOSHReleaseManifestsFromTarballs(tarballPaths ...string) ([]BOSHReleaseTarball, error) {
	results := make([]BOSHReleaseTarball, 0, len(tarballPaths))
	for _, tarballPath := range tarballPaths {
		tb, err := OpenBOSHReleaseTarball(tarballPath)
		if err != nil {
			return nil, err
		}
		results = append(results, tb)
	}
	return slices.Clip(results), nil
}

func OpenBOSHReleaseTarball(tarballPath string) (BOSHReleaseTarball, error) {
	file, err := os.Open(tarballPath)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	info, err := file.Stat()
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	if info.Size() == 0 {
		return BOSHReleaseTarball{}, fmt.Errorf("BOSH release tarball %s is an empty file", filepath.Base(tarballPath))
	}
	defer closeAndIgnoreError(file)
	return ReadBOSHReleaseTarball(tarballPath, file)
}

func ReadBOSHReleaseTarball(tarballPath string, r io.Reader) (BOSHReleaseTarball, error) {
	sum := sha1.New()
	r = io.TeeReader(r, sum)
	m, err := ReadProductTemplatePartFromBOSHReleaseTarball(r)
	if err != nil {
		return BOSHReleaseTarball{}, err
	}
	_, err = io.ReadAll(r)
	return BOSHReleaseTarball{
		Manifest: m,
		SHA1:     hex.EncodeToString(sum.Sum(nil)),
		FilePath: tarballPath,
	}, err
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
