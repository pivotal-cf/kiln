package models

type MetadataOut struct {
	Name                              string           `yaml:"name"`
	ProductVersion                    string           `yaml:"product_version"`
	IconImage                         string           `yaml:"icon_image"`
	Label                             string           `yaml:"label"`
	MetadataVersion                   string           `yaml:"metadata_version"`
	MinimumVersionForUpgrade          string           `yaml:"minimum_version_for_upgrade"`
	Rank                              int              `yaml:"rank"`
	Serial                            bool             `yaml:"serial"`
	PropertyBlueprints                []string         `yaml:"property_blueprints"`
	FormTypes                         []string         `yaml:"form_types"`
	Variables                         []string         `yaml:"variables"`
	InstanceGroups                    []string         `yaml:"job_types"`
	StemcellCriteria                  StemcellCriteria `yaml:"stemcell_criteria"`
	Releases                          []string         `yaml:"releases"`
	RuntimeConfigs                    []string         `yaml:"runtime_configs"`
	RequiresKubernetes                bool             `yaml:"requires_kubernetes"`
	CompatibleKubernetesDistributions []ProductVersion `yaml:"compatible_kubernetes_distributions"`
}

type StemcellCriteria struct {
	Os      string `yaml:"os"`
	Version string `yaml:"version"`
}

type ProductVersion struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}
