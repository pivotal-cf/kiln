package models

type Metadata struct {
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
	PackageInstalls                   []string         `yaml:"package_installs"`
	CompatibleKubernetesDistributions []ProductVersion `yaml:"compatible_kubernetes_distributions,omitempty"`
}
