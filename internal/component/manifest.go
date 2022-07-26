package component

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"path"
)

// ReadReleaseManifest reads from the tarball and parses out the manifest
func ReadReleaseManifest(releaseTarball io.Reader) ([]byte, error) {
	const releaseManifestFileName = "release.MF"
	zipReader, err := gzip.NewReader(releaseTarball)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(zipReader)

	for {
		h, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if path.Base(h.Name) != releaseManifestFileName {
			continue
		}
		return io.ReadAll(tarReader)
	}

	return nil, fmt.Errorf("%q not found", releaseManifestFileName)
}
