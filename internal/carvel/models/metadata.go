package models

import "github.com/pivotal-cf/kiln/pkg/proofing"

type Metadata struct {
	Name                              string                   `yaml:"name"`
	ProductVersion                    string                   `yaml:"product_version"`
	IconImage                         string                   `yaml:"icon_image"`
	Label                             string                   `yaml:"label"`
	MetadataVersion                   string                   `yaml:"metadata_version"`
	MinimumVersionForUpgrade          string                   `yaml:"minimum_version_for_upgrade"`
	Rank                              int                      `yaml:"rank"`
	Serial                            bool                     `yaml:"serial"`
	PropertyBlueprints                []string                 `yaml:"property_blueprints"`
	FormTypes                         []string                 `yaml:"form_types"`
	Variables                         []proofing.Variable      `yaml:"variables"`
	PackageInstalls                   []string                 `yaml:"package_installs"`
	CompatibleKubernetesDistributions []ProductVersion         `yaml:"compatible_kubernetes_distributions,omitempty"`
	SupportsParallelDeploys           bool                     `yaml:"supports_parallel_deploys,omitempty"`
	RequiresProductVersions           []RequiredProductVersion `yaml:"requires_product_versions,omitempty"`
	UsesKubernetesFeatures            []KubernetesFeature      `yaml:"uses_kubernetes_features,omitempty"`
}
