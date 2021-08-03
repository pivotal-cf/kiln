package builder

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"gopkg.in/yaml.v2"
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

type ReleaseManifestReader struct {
	fs billy.Filesystem
}

func NewReleaseManifestReader(fs billy.Filesystem) ReleaseManifestReader {
	return ReleaseManifestReader{fs: fs}
}

func (r ReleaseManifestReader) Read(releaseTarball string) (Part, error) {
	if r.fs == nil {
		r.fs = osfs.New("")
	}

	file, err := r.fs.Open(releaseTarball)
	if err != nil {
		return Part{}, err
	}
	defer func() { _ = file.Close() }()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return Part{}, err
	}
	defer func() { _ = gr.Close() }()

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
		File:            filepath.Base(releaseTarball),
		StemcellOS:      stemcellOS,
		StemcellVersion: stemcellVersion,
	}

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
		File:     releaseTarball,
		Name:     inputReleaseManifest.Name,
		Metadata: outputReleaseManifest,
	}, nil
}
