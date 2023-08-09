package builder

import (
	"fmt"
	"path/filepath"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type ReleaseManifest struct {
	Name            string
	Version         string
	File            string
	SHA1            string
	StemcellOS      string `yaml:"-"`
	StemcellVersion string `yaml:"-"`
}

type ReleaseManifestReader struct{}

func NewReleaseManifestReader() (_ ReleaseManifestReader) { return }

func (r ReleaseManifestReader) Read(releaseTarballFilepath string) (Part, error) {
	releaseTarball, err := cargo.ReadBOSHReleaseTarball(releaseTarballFilepath)
	if err != nil {
		return Part{}, err
	}

	stemcellOS, stemcellVersion, stemcellOK := releaseTarball.Manifest.Stemcell()
	if !stemcellOK && len(releaseTarball.Manifest.CompiledPackages) > 0 {
		return Part{}, fmt.Errorf("%s/%s has invalid stemcell: %q", releaseTarball.Manifest.Name, releaseTarball.Manifest.Version, releaseTarball.Manifest.CompiledPackages[0].Stemcell)
	}

	return Part{
		File: releaseTarballFilepath,
		Name: releaseTarball.Manifest.Name,
		Metadata: ReleaseManifest{
			Name:            releaseTarball.Manifest.Name,
			Version:         releaseTarball.Manifest.Version,
			File:            filepath.Base(releaseTarballFilepath),
			SHA1:            releaseTarball.SHA1,
			StemcellOS:      stemcellOS,
			StemcellVersion: stemcellVersion,
		},
	}, nil
}
