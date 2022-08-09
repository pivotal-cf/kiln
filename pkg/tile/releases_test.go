package tile_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/pivotal-cf/kiln/pkg/proofing"

	Ω "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/tile"
)

func TestReadReleaseFromFile(t *testing.T) {
	please := Ω.NewWithT(t)

	buf := bytes.NewBuffer(nil)
	releaseMetadata, err := tile.ReadReleaseFromFile("testdata/tile-0.1.2.pivotal", "hello-release", "v0.1.4", buf)
	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(releaseMetadata).To(Ω.Equal(proofing.Release{
		File:    "hello-release-v0.1.4-ubuntu-xenial-621.256.tgz",
		Name:    "hello-release",
		SHA1:    "c471ac6371eb8fc24508b14d9a49a44f9a5ef98c",
		Version: "v0.1.4",
	}))

	_, err = io.ReadAll(buf)
	please.Expect(err).NotTo(Ω.HaveOccurred())
}
