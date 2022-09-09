package manifest

import (
	opsman2 "github.com/pivotal-cf/kiln/internal/manifest/opsman"
	"github.com/pivotal-cf/kiln/internal/proofing"
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	. "github.com/onsi/gomega"
	gomegaMatchers "github.com/pivotal-cf-experimental/gomegamatchers"
)

func TestGenerate(t *testing.T) {
	t.Run("generates a well-formed manifest", func(t *testing.T) {
		please := NewWithT(t)

		f, err := os.Open("fixtures/metadata.yml")
		defer closeAndIgnoreError(f)
		please.Expect(err).NotTo(HaveOccurred())

		template, err := proofing.Parse(f)
		please.Expect(err).NotTo(HaveOccurred())

		manifest := Generate(template, OpsManagerConfig{
			DeploymentName: "some-product-name",
			AvailabilityZones: []string{
				"some-az-1",
				"some-az-2",
			},
			Stemcells: []opsman2.Stemcell{
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
			},
			ResourceConfigs: []opsman2.ResourceConfig{
				{
					Name:      "some-job-type-name",
					Instances: opsman2.ResourceConfigInstances{Value: 1},
				},
				{
					Name:      "other-job-type-name",
					Instances: opsman2.ResourceConfigInstances{Value: -1}, // NOTE: negative value indicates "automatic"
				},
			},
		})

		actualManifest, err := yaml.Marshal(manifest)
		please.Expect(err).NotTo(HaveOccurred())

		expectedManifest, err := os.ReadFile("fixtures/manifest.yml")
		please.Expect(err).NotTo(HaveOccurred())

		please.Expect(actualManifest).To(gomegaMatchers.HelpfullyMatchYAML(string(expectedManifest)))
	})
}
