package manifest

import (
	"os"
	"testing"

	"gopkg.in/yaml.v2"

	. "github.com/onsi/gomega"
	gomegaMatchers "github.com/pivotal-cf-experimental/gomegamatchers"

	"github.com/pivotal-cf/kiln/pkg/manifest/opsman"
	"github.com/pivotal-cf/kiln/pkg/proofing"
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
			Stemcells: []opsman.Stemcell{
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
			ResourceConfigs: []opsman.ResourceConfig{
				{
					Name:      "some-job-type-name",
					Instances: opsman.ResourceConfigInstances{Value: 1},
				},
				{
					Name:      "other-job-type-name",
					Instances: opsman.ResourceConfigInstances{Value: -1}, // NOTE: negative value indicates "automatic"
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
