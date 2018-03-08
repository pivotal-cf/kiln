package proofing

type ProductTemplate struct {
	Name                     string `yaml:"name"`
	ProductVersion           string `yaml:"product_version"`
	MinimumVersionForUpgrade string `yaml:"minimum_version_for_upgrade"`
	Label                    string `yaml:"label"`
	Rank                     int    `yaml:"rank"`
	MetadataVersion          string `yaml:"metadata_version"`
	OriginalMetadataVersion  string `yaml:"original_metadata_version"`
	ServiceBroker            bool   `yaml:"service_broker"`
	BaseReleasesURL          string `yaml:"base_releases_url"`
	Cloud                    string `yaml:"cloud"`
	Network                  string `yaml:"network"`

	IconImage           string `yaml:"icon_image"`
	DeprecatedTileImage string `yaml:"deprecated_tile_image"`

	Serial                  bool                     `yaml:"serial"`
	InstallTimeVerifiers    []InstallTimeVerifier    `yaml:"install_time_verifiers"`
	Variables               []Variable               `yaml:"variables"`
	Releases                []Release                `yaml:"releases"`
	StemcellCriteria        StemcellCriteria         `yaml:"stemcell_criteria"`
	PropertyBlueprints      PropertyBlueprints       `yaml:"property_blueprints"`
	FormTypes               []FormType               `yaml:"form_types"`
	JobTypes                []JobType                `yaml:"job_types"`
	PostDeployErrands       []ErrandTemplate         `yaml:"post_deploy_errands"`
	PreDeleteErrands        []ErrandTemplate         `yaml:"pre_delete_errands"`
	RuntimeConfigs          []RuntimeConfigTemplate  `yaml:"runtime_configs"`
	RequiresProductVersions []RequiresProductVersion `yaml:"requires_product_versions"`

	// TODO: unused?
	Description             string                   `yaml:"description"`
	ProvidesProductVersions []ProvidesProductVersion `yaml:"provides_product_versions"`

	// TODO: version_attribute: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L30-L32
	// TODO: validates_presence_of: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L20-L25
}
