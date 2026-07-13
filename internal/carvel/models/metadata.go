package models

import "github.com/pivotal-cf/kiln/pkg/proofing"

type Metadata struct {
	Name                              string               `yaml:"name"`
	ProductVersion                    string               `yaml:"product_version"`
	IconImage                         string               `yaml:"icon_image"`
	Label                             string               `yaml:"label"`
	MetadataVersion                   string               `yaml:"metadata_version"`
	MinimumVersionForUpgrade          string               `yaml:"minimum_version_for_upgrade"`
	Rank                              int                  `yaml:"rank"`
	Serial                            bool                 `yaml:"serial"`
	PropertyBlueprints                []string             `yaml:"property_blueprints"`
	FormTypes                         []string             `yaml:"form_types"`
	Variables                         []proofing.Variable  `yaml:"variables"`
	PackageInstalls                   []string             `yaml:"package_installs"`
	CompatibleKubernetesDistributions []ProductVersion     `yaml:"compatible_kubernetes_distributions,omitempty"`
	// AdditionalReleases declares locally-built BOSH releases that kiln should
	// splice into the runtime-config addon alongside the auto-generated tile
	// release. Each entry's TarballPath is resolved relative to the tile source
	// directory; kiln copies it verbatim into .carvel-tile/releases/ and adds
	// the release + declared jobs to the runtime-config addon's releases:/jobs:
	// lists. The tile's own build.sh is responsible for producing the tarball
	// before invoking kiln carvel bake.
	AdditionalReleases []AdditionalRelease `yaml:"additional_releases,omitempty"`
}

// AdditionalRelease declares a locally-built BOSH release to include alongside
// the auto-generated tile release in the carvel runtime-config addon.
type AdditionalRelease struct {
	// Name is the BOSH release name (must match the name used in bosh create-release).
	Name string `yaml:"name"`
	// Version is the BOSH release version (e.g. "dev" for POC builds).
	Version string `yaml:"version"`
	// TarballPath is the path to the pre-built release tarball, relative to
	// the tile source directory.
	TarballPath string `yaml:"tarball_path"`
	// Jobs lists the job names from this release to add to the addon's jobs: list.
	Jobs []string `yaml:"jobs"`
}
