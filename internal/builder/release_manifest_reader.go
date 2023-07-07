package builder

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"

	"github.com/pivotal-cf/kiln/pkg/cargo"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
)

type ReleaseManifest struct {
	Name            string
	Version         string
	File            string
	SHA1            string
	StemcellOS      string `yaml:"-"`
	StemcellVersion string `yaml:"-"`
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
	defer closeAndIgnoreError(file)

	inputReleaseManifest, err := cargo.ReadProductTemplatePartFromBOSHReleaseTarball(file)
	if err != nil {
		return Part{}, err
	}

	stemcellOS, stemcellVersion, stemcellOK := inputReleaseManifest.Stemcell()
	if !stemcellOK && len(inputReleaseManifest.CompiledPackages) > 0 {
		return Part{}, fmt.Errorf("%s/%s has invalid stemcell: %q", inputReleaseManifest.Name, inputReleaseManifest.Version, inputReleaseManifest.CompiledPackages[0].Stemcell)
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

	outputReleaseManifest.SHA1 = hex.EncodeToString(hash.Sum(nil))

	return Part{
		File:     releaseTarball,
		Name:     inputReleaseManifest.Name,
		Metadata: outputReleaseManifest,
	}, nil
}
