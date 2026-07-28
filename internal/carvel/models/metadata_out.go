package models

import "github.com/pivotal-cf/kiln/pkg/proofing"

type MetadataOut struct {
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
	InstanceGroups                    []string                 `yaml:"job_types"`
	StemcellCriteria                  StemcellCriteria         `yaml:"stemcell_criteria"`
	Releases                          []string                 `yaml:"releases"`
	RuntimeConfigs                    []string                 `yaml:"runtime_configs"`
	RequiresKubernetes                bool                     `yaml:"requires_kubernetes"`
	CompatibleKubernetesDistributions []ProductVersion         `yaml:"compatible_kubernetes_distributions"`
	SupportsParallelDeploys           bool                     `yaml:"supports_parallel_deploys,omitempty"`
	RequiresProductVersions           []RequiredProductVersion `yaml:"requires_product_versions,omitempty"`
}

type StemcellCriteria struct {
	Os      string `yaml:"os"`
	Version string `yaml:"version"`
}

type ProductVersion struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type RequiredProductVersion struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	Optional bool   `yaml:"optional,omitempty"`
}
