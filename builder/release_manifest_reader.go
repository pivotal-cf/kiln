package builder

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type ReleaseManifest struct {
	Name            string
	Version         string
	File            string
	SHA1            string
	StemcellOS      string `yaml:"-"`
	StemcellVersion string `yaml:"-"`
}

type inputReleaseManifest struct {
	Name             string            `yaml:"name"`
	Version          string            `yaml:"version"`
	CompiledPackages []compiledPackage `yaml:"compiled_packages"`
}

type compiledPackage struct {
	Stemcell string `yaml:"stemcell"`
}

type ReleaseManifestReader struct{}

func NewReleaseManifestReader() ReleaseManifestReader {
	return ReleaseManifestReader{}
}

func (r ReleaseManifestReader) Read(releaseTarball string) (Part, error) {
	file, err := os.Open(releaseTarball)
	if err != nil {
		return Part{}, err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return Part{}, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	var header *tar.Header
	for {
		header, err = tr.Next()
		if err != nil {
			if err == io.EOF {
				return Part{}, fmt.Errorf("could not find release.MF in %q", releaseTarball)
			}

			return Part{}, fmt.Errorf("error while reading %q: %s", releaseTarball, err)
		}

		if filepath.Base(header.Name) == "release.MF" {
			break
		}
	}

	var inputReleaseManifest inputReleaseManifest
	inputReleaseManifestContents, err := ioutil.ReadAll(tr)
	if err != nil {
		return Part{}, err // NOTE: cannot replicate this error scenario in a test
	}

	err = yaml.Unmarshal(inputReleaseManifestContents, &inputReleaseManifest)
	if err != nil {
		return Part{}, err
	}

	var stemcellOS, stemcellVersion string
	compiledPackages := inputReleaseManifest.CompiledPackages
	if len(compiledPackages) > 0 {
		inputStemcell := inputReleaseManifest.CompiledPackages[0].Stemcell
		stemcellParts := strings.Split(inputStemcell, "/")
		if len(stemcellParts) != 2 {
			return Part{}, fmt.Errorf("Invalid format for compiled package stemcell inside release.MF (expected 'os/version'): %s", inputStemcell)
		}
		stemcellOS = stemcellParts[0]
		stemcellVersion = stemcellParts[1]
	}

	outputReleaseManifest := ReleaseManifest{
		Name:            inputReleaseManifest.Name,
		Version:         inputReleaseManifest.Version,
		StemcellOS:      stemcellOS,
		StemcellVersion: stemcellVersion,
	}

	outputReleaseManifest.File = filepath.Base(releaseTarball)

	_, err = file.Seek(0, 0)
	if err != nil {
		return Part{}, err // NOTE: cannot replicate this error scenario in a test
	}

	hash := sha1.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return Part{}, err // NOTE: cannot replicate this error scenario in a test
	}

	outputReleaseManifest.SHA1 = fmt.Sprintf("%x", hash.Sum(nil))

	return Part{
		Name:     inputReleaseManifest.Name,
		Metadata: outputReleaseManifest,
	}, nil
}
