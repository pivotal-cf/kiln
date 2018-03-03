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

	IconImage           string `yaml:"icon_image"`
	DeprecatedTileImage string `yaml:"deprecated_tile_image"`

	Serial               bool                    `yaml:"serial"`
	InstallTimeVerifiers []InstallTimeVerifier   `yaml:"install_time_verifiers"`
	Variables            []Variable              `yaml:"variables"`
	Releases             []Release               `yaml:"releases"`
	StemcellCriteria     StemcellCriteria        `yaml:"stemcell_criteria"`
	PropertyBlueprints   []PropertyBlueprint     `yaml:"property_blueprints"`
	FormTypes            []FormType              `yaml:"form_types"`
	JobTypes             []JobType               `yaml:"job_types"`
	PostDeployErrands    []ErrandTemplate        `yaml:"post_deploy_errands"`
	RuntimeConfigs       []RuntimeConfigTemplate `yaml:"runtime_configs"`

	// TODO: unused?
	Description             string                   `yaml:"description"`
	ProvidesProductVersions []ProvidesProductVersion `yaml:"provides_product_versions"`

	// TODO: property_blueprints can be of differing types: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/property_blueprint_builder.rb
	// TODO: validates_presence_of: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L20-L25
	// TODO: version_attribute: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L30-L32
	// TODO: cloud and network: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L36-L37
	// TODO: base_releases_url: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L43
	// TODO: requires_product_versions: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L50
	// TODO: pre_delete_errands: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L52
}
