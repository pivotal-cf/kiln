package tile

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

func ReadReleaseFromFile(tilePath, releaseName, releaseVersion string, releaseTarball io.Writer) (proofing.Release, error) {
	f, err := os.Open(tilePath)
	if err != nil {
		return proofing.Release{}, err
	}
	defer closeAndIgnoreError(f)
	fi, err := f.Stat()
	if err != nil {
		return proofing.Release{}, err
	}
	return ReadReleaseFromZip(f, fi.Size(), releaseName, releaseVersion, releaseTarball)
}

func ReadReleaseFromZip(ra io.ReaderAt, zipFileSize int64, releaseName, releaseVersion string, releaseTarball io.Writer) (proofing.Release, error) {
	zr, err := zip.NewReader(ra, zipFileSize)
	if err != nil {
		return proofing.Release{}, fmt.Errorf("failed to do open metadata zip reader: %w", err)
	}
	return ReadReleaseFromFS(zr, releaseName, releaseVersion, releaseTarball)
}

func ReadReleaseFromFS(dir fs.FS, releaseName, releaseVersion string, releaseTarball io.Writer) (proofing.Release, error) {
	metadataBuf, err := ReadMetadataFromFS(dir)
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
