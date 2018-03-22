package cargo_test

import (
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/internal/cargo/bosh"
	"github.com/pivotal-cf/kiln/internal/proofing"

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
			template := proofing.ProductTemplate{
				Releases: []proofing.Release{
					{
						Name:    "some-release-name",
						Version: "some-release-version",
					},
				},
				StemcellCriteria: proofing.StemcellCriteria{
					OS:      "some-stemcell-os",
					Version: "some-stemcell-version",
				},
				Serial: true,
			}

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

			manifest := generator.Execute("some-product-name", template, stemcells)
			Expect(manifest).To(Equal(cargo.Manifest{
				Name: "some-product-name",
				Releases: []cargo.Release{
					{
						Name:    "some-release-name",
						Version: "some-release-version",
					},
				},
				Stemcells: []cargo.Stemcell{
					{
						Alias:   "some-stemcell-name",
						OS:      "some-stemcell-os",
						Version: "some-stemcell-version",
					},
				},
				Update: cargo.Update{
					Canaries:        1,
					CanaryWatchTime: "30000-300000",
					UpdateWatchTime: "30000-300000",
					MaxInFlight:     1,
					MaxErrors:       2,
					Serial:          true,
				},
			}))
		})
	})
})
