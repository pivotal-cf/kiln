package builder

import (
	"github.com/pivotal-cf/kiln/pkg/proofing"
	"path/filepath"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type ReleaseManifestReader struct{}

func NewReleaseManifestReader() (_ ReleaseManifestReader) { return }

func (r ReleaseManifestReader) Read(releaseTarballFilepath string) (Part, error) {
	releaseTarball, err := cargo.ReadBOSHReleaseTarball(releaseTarballFilepath)
	if err != nil {
		return Part{}, err
	}

	return Part{
		File: releaseTarballFilepath,
		Name: releaseTarball.Manifest.Name,
		Metadata: proofing.Release{
			Name:       releaseTarball.Manifest.Name,
			Version:    releaseTarball.Manifest.Version,
			File:       filepath.Base(releaseTarballFilepath),
			SHA1:       releaseTarball.SHA1,
			CommitHash: releaseTarball.Manifest.CommitHash,
		},
	}, nil
}
