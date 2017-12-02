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

type ReleaseManifest struct {
	Name    string
	Version string
}

type ReleaseManifestReader struct {
	filesystem filesystem
}

func NewReleaseManifestReader(filesystem filesystem) ReleaseManifestReader {
	return ReleaseManifestReader{
		filesystem: filesystem,
	}
}

func (r ReleaseManifestReader) Read(releaseTarball string) (ReleaseManifest, error) {
	file, err := r.filesystem.Open(releaseTarball)
	if err != nil {
		return ReleaseManifest{}, err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return ReleaseManifest{}, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return ReleaseManifest{}, fmt.Errorf("could not find release.MF in %q", releaseTarball)
			}

			return ReleaseManifest{}, fmt.Errorf("error while reading %q: %s", releaseTarball, err)
		}

		if filepath.Base(header.Name) == "release.MF" {
			break
		}
	}

	var releaseManifest ReleaseManifest
	releaseContent, err := ioutil.ReadAll(tr)
	if err != nil {
		return ReleaseManifest{}, err
	}

	err = yaml.Unmarshal(releaseContent, &releaseManifest)
	if err != nil {
		return ReleaseManifest{}, err
	}

	return releaseManifest, nil
}
