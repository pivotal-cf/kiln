package manifest_generator_test

import (
	"fmt"

	"github.com/pivotal-cf/kiln/builder"
	. "github.com/pivotal-cf/kiln/internal/manifest_generator"
	"github.com/pivotal-cf/kiln/release"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ManifestGenerator", func() {
	var (
		desiredReleases []release.ID
		desiredStemcell builder.StemcellManifest
	)

	const (
		deploymentName = "lets-make-mashed-potatoes"
	)

	BeforeEach(func() {
		desiredReleases = []release.ID{
			{Name: "potato-release", Version: "9000.0.1"},
			{Name: "rutabaga-release", Version: "42.0.0"},
		}

		desiredStemcell = builder.StemcellManifest{
			OperatingSystem: "michaelsoft-vind0ez",
			Version:         "1.0.0",
		}
	})

	It("creates a manifest with the given releases and stemcell", func() {
		generator := NewManifestGenerator()
		manifest, err := generator.Generate(
			deploymentName,
			desiredReleases,
			desiredStemcell,
		)
		Expect(err).NotTo(HaveOccurred())

		expectedYAML := fmt.Sprintf(`---
name: %q
releases:
  - name: %q
    version: %q
  - name: %q
    version: %q
stemcells:
  - alias: default
    os: %q
    version: %q
update:
  canaries: 1
  max_in_flight: 1
  canary_watch_time: 1000-1001
  update_watch_time: 1000-1001
instance_groups: []
`, deploymentName,
			desiredReleases[0].Name, desiredReleases[0].Version,
			desiredReleases[1].Name, desiredReleases[1].Version,
			desiredStemcell.OperatingSystem, desiredStemcell.Version)

		Expect(manifest).To(MatchYAML(expectedYAML))
	})
})
