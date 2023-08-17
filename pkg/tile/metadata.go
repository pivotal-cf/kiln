package tile

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"

	bufferReaderAt "github.com/avvmoto/buf-readerat"
	"github.com/snabb/httpreaderat"
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
	const pattern = "metadata/*.yml"
	matches, err := fs.Glob(dir, pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("metadata file not found in the tile: expected a file matching glob %q", pattern)
	}
	metadataFile, err := dir.Open(matches[0])
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

const (
	// readerAtCacheSize is 1mb
	readerAtCacheSize = 1 << 20
)

// ReadMetadataFromProductFile can download the metadata from a product file on TanzuNet.
func ReadMetadataFromProductFile(client *http.Client, req *http.Request) ([]byte, error) {
	ra, err := httpreaderat.New(client, req, nil)
	if err != nil {
		return nil, err
	}
	bufRa := bufferReaderAt.NewBufReaderAt(ra, readerAtCacheSize)
	return ReadMetadataFromZip(bufRa, ra.Size())
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}
