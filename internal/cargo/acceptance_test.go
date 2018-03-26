package cargo_test

import (
	"io/ioutil"

	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/internal/cargo/bosh"
	"github.com/pivotal-cf/kiln/internal/proofing"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Acceptance", func() {
	var generator cargo.Generator

	BeforeEach(func() {
		generator = cargo.NewGenerator()
	})

	XIt("can render a PAS manifest", func() {
		productTemplate, err := proofing.Parse("fixtures/acceptance/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		stemcells := []bosh.Stemcell{}

		manifest := generator.Execute("cf-1234", productTemplate, stemcells)

		actualManifest, err := yaml.Marshal(manifest)
		Expect(err).NotTo(HaveOccurred())

		expectedManifest, err := ioutil.ReadFile("fixtures/acceptance/manifest.yml")
		Expect(err).NotTo(HaveOccurred())
		Expect(actualManifest).To(MatchYAML(string(expectedManifest)))
	})
})
