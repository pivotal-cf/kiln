package test_helpers

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
)

func WriteReleaseTarball(path, name, version string, fs billy.Filesystem) (string, error) {
	releaseManifest := `
name: ` + name + `
version: ` + version + `
`
	return WriteTarballWithFile(path, "release.MF", releaseManifest, fs)
}

func WriteTarballWithFile(tarballPath, internalFilePath, fileContents string, fs billy.Filesystem) (string, error) {
	f, err := fs.Create(tarballPath)
	if err != nil {
		return "", err
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	contentsReader := strings.NewReader(fileContents)

	header := &tar.Header{
		Name:    internalFilePath,
		Size:    contentsReader.Size(),
		Mode:    int64(os.O_RDONLY),
		ModTime: time.Now(),
	}
	err = tw.WriteHeader(header)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(tw, contentsReader)
	if err != nil {
		return "", err
	}

	err = tw.Close()
	if err != nil {
		return "", err
	}

	err = gw.Close()
	if err != nil {
		return "", err
	}

	err = f.Close()
	if err != nil {
		return "", err
	}

	tarball, err := fs.Open(tarballPath)
	if err != nil {
		return "", err
	}
	defer closeAndIgnoreError(tarball)

	hash := sha1.New()
	_, err = io.Copy(hash, tarball)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
