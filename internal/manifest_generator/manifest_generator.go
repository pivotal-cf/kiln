package manifest_generator

import (
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/pkg/component"
)

type ManifestGenerator struct{}

type Manifest struct {
	Name           string             `yaml:"name"`
	Releases       []ManifestRelease  `yaml:"releases"`
	Stemcells      []ManifestStemcell `yaml:"stemcells"`
	Update         ManifestUpdate     `yaml:"update"`
	InstanceGroups []interface{}      `yaml:"instance_groups"`
}

type ManifestRelease struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type ManifestStemcell struct {
	Alias   string `yaml:"alias"`
	OS      string `yaml:"os"`
	Version string `yaml:"version"`
}

type ManifestUpdate struct {
	Canaries        int    `yaml:"canaries"`
	MaxInFlight     int    `yaml:"max_in_flight"`
	CanaryWatchTime string `yaml:"canary_watch_time"`
	UpdateWatchTime string `yaml:"update_watch_time"`
}

func New() ManifestGenerator {
	return ManifestGenerator{}
}

func (g ManifestGenerator) Generate(deploymentName string, releases []component.Spec, stemcell builder.StemcellManifest) ([]byte, error) {
	manifest := Manifest{
		Name:           deploymentName,
		Releases:       []ManifestRelease{},
		Stemcells:      []ManifestStemcell{},
		InstanceGroups: nil,
		Update: ManifestUpdate{
			Canaries:        1,
			MaxInFlight:     1,
			CanaryWatchTime: "1000-1001",
			UpdateWatchTime: "1000-1001",
		},
	}

	for _, rel := range releases {
		manifest.Releases = append(
			manifest.Releases,
			ManifestRelease{
				Name:    rel.Name,
				Version: rel.Version,
			},
		)
	}
	manifest.Stemcells = []ManifestStemcell{
		{
			Alias:   "default",
			OS:      stemcell.OperatingSystem,
			Version: stemcell.Version,
		},
	}

	manifestYAML, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	return manifestYAML, nil
}
