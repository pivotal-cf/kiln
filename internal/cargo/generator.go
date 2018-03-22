package cargo

import (
	"github.com/pivotal-cf/kiln/internal/cargo/bosh"
	"github.com/pivotal-cf/kiln/internal/proofing"
)

type Generator struct{}

func NewGenerator() Generator {
	return Generator{}
}

func (g Generator) Execute(name string, template proofing.ProductTemplate, boshStemcells []bosh.Stemcell) Manifest {
	var releases []Release
	for _, release := range template.Releases {
		releases = append(releases, Release{
			Name:    release.Name,
			Version: release.Version,
		})
	}

	var stemcells []Stemcell
	for _, boshStemcell := range boshStemcells {
		if boshStemcell.OS == template.StemcellCriteria.OS {
			if boshStemcell.Version == template.StemcellCriteria.Version {
				stemcells = append(stemcells, Stemcell{
					Alias:   boshStemcell.Name,
					OS:      boshStemcell.OS,
					Version: boshStemcell.Version,
				})
			}
		}
	}

	update := Update{
		Canaries:        1,
		CanaryWatchTime: "30000-300000",
		UpdateWatchTime: "30000-300000",
		MaxInFlight:     1,
		MaxErrors:       2,
		Serial:          template.Serial,
	}

	return Manifest{
		Name:      name,
		Releases:  releases,
		Stemcells: stemcells,
		Update:    update,
	}
}
