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

var _ = Describe("Generator", func() {
	var generator cargo.Generator

	BeforeEach(func() {
		generator = cargo.NewGenerator()
	})

	Describe("Execute", func() {
		It("generates a well-formed manifest", func() {
			template, err := proofing.Parse("fixtures/metadata.yml")
			Expect(err).NotTo(HaveOccurred())

			stemcells := []bosh.Stemcell{
				{
					Name:    "some-stemcell-name",
					Version: "some-stemcell-version",
					OS:      "some-stemcell-os",
				},
				{
					Name:    "other-stemcell-name",
					Version: "other-stemcell-version",
					OS:      "other-stemcell-os",
				},
			}

			availabilityZones := []string{
				"some-az-1",
				"some-az-2",
			}

			manifest := generator.Execute("some-product-name", template, stemcells, availabilityZones)

			actualManifest, err := yaml.Marshal(manifest)
			Expect(err).NotTo(HaveOccurred())

			expectedManifest, err := ioutil.ReadFile("fixtures/manifest.yml")
			Expect(err).NotTo(HaveOccurred())

			Expect(actualManifest).To(MatchYAML(string(expectedManifest)))
		})
	})
})
