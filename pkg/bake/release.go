package bake

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type release struct {
	// these fields must be ordered alphabetically for tests to pass
	File    string `json:"file"`
	Name    string `json:"name"`
	SHA1    string `json:"sha1"`
	Version string `json:"version"`
}

func releaseFromKilnfile(kilnfileLock cargo.KilnfileLock, name string) (release, error) {
	lock, err := kilnfileLock.FindReleaseWithName(name)
	if err != nil {
		return release{}, fmt.Errorf("failed to find release %s in Kilnfile: %w", name, err)
	}
	localFilePath := path.Base(lock.RemotePath)
	if lock.RemoteSource == cargo.ReleaseSourceTypeBOSHIO {
		localFilePath = fmt.Sprintf("%s-%v.tgz", lock.Name, lock.Version)
	}
	return release{
		Name:    lock.Name,
		Version: lock.Version,
		SHA1:    lock.SHA1,
		File:    localFilePath,
	}, err
}

func newReleasesFromDirectories(tileDir fs.FS, directories []string) ([]release, error) {
	var result []release
	for _, dir := range directories {
		files, err := fs.ReadDir(tileDir, dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, file := range files {
			rel, err := newReleaseFromFile(tileDir, path.Join(dir, file.Name()))
			if err != nil {
				return nil, err
			}
			result = append(result, rel)
		}
	}
	return result, nil
}

func newReleaseFromFile(tileDir fs.FS, filePath string) (release, error) {
	rel, err := readNameAndVersionFromManifest(tileDir, filePath)
	if err != nil {
		return release{}, err
	}
	sum, err := readSHA(tileDir, filePath)
	if err != nil {
		return release{}, err
	}
	rel.SHA1 = sum
	rel.File = path.Base(filePath)
	return rel, nil
}

func readSHA(tileDir fs.FS, filePath string) (string, error) {
	f, err := tileDir.Open(filePath)
	if err != nil {
		return "", err
	}
	defer closeAndIgnoreError(f)
	hash := sha1.New()
	_, err = io.Copy(hash, f)
	if err != nil {
		return "", err // NOTE: cannot replicate this error scenario in a test
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func readNameAndVersionFromManifest(tileDir fs.FS, filePath string) (release, error) {
	f, err := tileDir.Open(filePath)
	if err != nil {
		return release{}, err
	}
	defer closeAndIgnoreError(f)

	var tr *tar.Reader
	switch path.Ext(filePath) {
	case ".tgz", ".gz":
		gzf, err := gzip.NewReader(f)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		tr = tar.NewReader(gzf)
	case ".tar":
		tr = tar.NewReader(f)
	default:
		return release{}, fmt.Errorf("unexpected file extention %q on file %q", path.Ext(filePath), filePath)
	}

	type compiledPackage struct {
		Stemcell string `yaml:"stemcell"`
	}

	var manifest struct {
		Name             string            `yaml:"name"`
		Version          string            `yaml:"version"`
		CompiledPackages []compiledPackage `yaml:"compiled_packages"`
	}

	for {
		ht, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return release{}, err
		}
		if strings.ToLower(path.Base(ht.Name)) != "release.mf" {
			continue
		}
		manifestBuf, err := io.ReadAll(tr)
		if err != nil {
			return release{}, fmt.Errorf("failed to release manifest for %s: %w", filePath, err)
		}
		err = yaml.Unmarshal(manifestBuf, &manifest)
		if err != nil {
			return release{}, fmt.Errorf("failed to parse manifest for %s: %w", filePath, err)
		}
	}
	return release{
		Name:    manifest.Name,
		Version: manifest.Version,
	}, nil
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}
