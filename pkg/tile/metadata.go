package tile

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
)

func ReadMetadataFromFile(tilePath string) ([]byte, error) {
	f, err := os.Open(tilePath)
	if err != nil {
		return nil, err
	}
	defer closeAndIgnoreError(f)
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
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
	defer closeAndIgnoreError(metadataFile)
	buf, err := io.ReadAll(metadataFile)
	if err != nil {
		return nil, fmt.Errorf("failed read metadata: %w", err)
	}
	return buf, nil
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}
