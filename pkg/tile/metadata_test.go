package tile_test

import (
	"testing"
	"testing/fstest"

	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/tile"
)

func TestReadMetadataFromFile(t *testing.T) {
	please := NewWithT(t)

	metadataBytes, err := tile.ReadMetadataFromFile("testdata/tile-0.1.2.pivotal")
	please.Expect(err).NotTo(HaveOccurred())

	var metadata struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(metadataBytes, &metadata)
	please.Expect(err).NotTo(HaveOccurred(), string(metadataBytes))

	please.Expect(metadata.Name).To(Equal("hello"), string(metadataBytes))
}

func TestNonStandardMetadataFilename(t *testing.T) {
	fileFS := fstest.MapFS{
		"metadata/banana.yml": &fstest.MapFile{Data: []byte(`{name: "banana"}`)},
	}
	buf, err := tile.ReadMetadataFromFS(fileFS)
	please := NewWithT(t)
	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(string(buf)).To(Equal(`{name: "banana"}`))
}
