package builder

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type StemcellManifest struct {
	Version         string
	OperatingSystem string `yaml:"operating_system"`
}

type StemcellManifestReader struct {
	filesystem filesystem
}

func NewStemcellManifestReader(filesystem filesystem) StemcellManifestReader {
	return StemcellManifestReader{
		filesystem: filesystem,
	}
}

func (r StemcellManifestReader) Read(stemcellTarball string) (StemcellManifest, error) {
	file, err := r.filesystem.Open(stemcellTarball)
	if err != nil {
		return StemcellManifest{}, err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return StemcellManifest{}, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	header, err := tr.Next()
	for err == nil {
		if filepath.Base(header.Name) == "stemcell.MF" {
			break
		}

		header, err = tr.Next()
	}
	if err != nil {
		if err == io.EOF {
			return StemcellManifest{}, fmt.Errorf("could not find stemcell.MF in %q", stemcellTarball)
		}

		return StemcellManifest{}, fmt.Errorf("error while reading %q: %s", stemcellTarball, err)
	}

	var stemcellManifest StemcellManifest
	stemcellContent, err := ioutil.ReadAll(tr)
	if err != nil {
		return StemcellManifest{}, err
	}

	err = yaml.Unmarshal(stemcellContent, &stemcellManifest)
	if err != nil {
		return StemcellManifest{}, err
	}

	return stemcellManifest, nil
}
