package tile

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

func ReadMetadataFromFile(tilePath string) ([]byte, error) {
	fi, err := os.Stat(tilePath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(tilePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	return ReadMetadataFromZip(f, fi.Size())
}

func ReadMetadataFromZip(ra io.ReaderAt, zipFileSize int64) ([]byte, error) {
	zr, err := zip.NewReader(ra, zipFileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to do open metadata zip reader: %w", err)
	}
	return ReadMetadataFromFS(zr)
}

func ReadMetadataFromFS(dir fs.FS) ([]byte, error) {
	metadataFile, err := dir.Open("metadata/metadata.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to do open metadata zip file: %w", err)
	}
	defer func() {
		_ = metadataFile.Close()
	}()
	buf, err := io.ReadAll(metadataFile)
	if err != nil {
		return nil, fmt.Errorf("failed read metadata: %w", err)
	}
	return buf, nil
}

func ReadTileMetadataAndReleaseMetadataFromZip(ra io.ReaderAt, zipFileSize int64) ([]byte, error) {
	zr, err := zip.NewReader(ra, zipFileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to do open metadata zip reader: %w", err)
	}
	return ReadTileMetadataAndReleaseMetadataFromFS(zr)
}

func ReadTileMetadataAndReleaseMetadataFromFS(dir fs.FS) ([]byte, error) {
	metadataFile, err := dir.Open("metadata/metadata.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to do open metadata zip file: %w", err)
	}
	defer func() {
		_ = metadataFile.Close()
	}()
	buf, err := io.ReadAll(metadataFile)
	if err != nil {
		return nil, fmt.Errorf("failed read metadata: %w", err)
	}

	releases, err := fs.ReadDir(dir, "releases")
	if err != nil {
		return nil, fmt.Errorf("failed read releases directory: %w", err)
	}
	for _, releaseFileInfo := range releases {
		releaseFile, err := dir.Open(path.Join("releases", releaseFileInfo.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read release file %q: %w", releaseFileInfo.Name(), err)
		}
		releaseMF, err := ReadReleaseManifest(releaseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read release file %q: %w", releaseFileInfo.Name(), err)
		}
		buf = append(buf, append([]byte("\n---\n"), releaseMF...)...)
	}
	return buf, nil
}

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
