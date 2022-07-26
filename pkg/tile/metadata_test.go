package tile_test

import (
	"testing"

	Ω "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/tile"
)

func TestReadMetadataFromFile(t *testing.T) {
	please := Ω.NewWithT(t)

	metadataBytes, err := tile.ReadMetadataFromFile("testdata/tile-0.1.2.pivotal")
	please.Expect(err).NotTo(Ω.HaveOccurred())

	var metadata struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(metadataBytes, &metadata)
	please.Expect(err).NotTo(Ω.HaveOccurred(), string(metadataBytes))

	please.Expect(metadata.Name).To(Ω.Equal("hello"), string(metadataBytes))
}
