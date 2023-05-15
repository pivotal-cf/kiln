package cargo

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"hash"
	"io"
	"io/fs"
	"strings"

	boshrelease "github.com/cloudfoundry/bosh-cli/release/manifest"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

func BOSHReleaseTileMetadataFromGzippedTarball(releasesDirectory fs.FS, filePath string) (proofing.Release, error) {
	checkSum := sha1.New()
	if err := fileChecksum(releasesDirectory, filePath, checkSum); err != nil {
		return proofing.Release{}, err
	}

	releaseMF, _, err := BOSHReleaseManifestAndLicense(releasesDirectory, filePath)
	if err != nil {
		return proofing.Release{}, err
	}

	return proofing.Release{
		Name:    releaseMF.Name,
		Version: releaseMF.Version,
		File:    filePath,
		SHA1:    hex.EncodeToString(checkSum.Sum(nil)),
	}, nil
}

func BOSHReleaseManifestAndLicense(releasesDirectory fs.FS, filePath string) (boshrelease.Manifest, []byte, error) {
	const (
		releaseMFFileName = "release.MF"
		licenseFileName   = "LICENSE"
	)
	f, err := releasesDirectory.Open(filePath)
	if err != nil {
		return boshrelease.Manifest{}, nil, err
	}
	defer closeAndIgnoreError(f)
	fileContents, err := readTarballFiles(f, releaseMFFileName, licenseFileName)
	if err != nil {
		return boshrelease.Manifest{}, nil, err
	}
	var releaseMF boshrelease.Manifest
	return releaseMF, fileContents[licenseFileName], yaml.Unmarshal(fileContents[releaseMFFileName], &releaseMF)
}

func fileChecksum(dir fs.FS, fileName string, hash hash.Hash) error {
	f, err := dir.Open(fileName)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(f)
	_, err = io.Copy(hash, f)
	return err
}

func readTarballFiles(r io.Reader, fileNamesToRead ...string) (map[string][]byte, error) {
	m := make(map[string][]byte, len(fileNamesToRead))
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer closeAndIgnoreError(gzipReader)
	tarball := tar.NewReader(gzipReader)
	for {
		h, err := tarball.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		fileName := cleanReleaseTarballFileName(h.Name)
		if slices.Index(fileNamesToRead, fileName) < 0 {
			continue
		}
		buf, err := io.ReadAll(tarball)
		if err != nil {
			return nil, err
		}
		m[fileName] = buf
	}
	return m, nil
}

func cleanReleaseTarballFileName(name string) string {
	return strings.TrimPrefix(strings.TrimPrefix(name, "/"), "./")
}
