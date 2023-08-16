package proofing

import (
	"fmt"
	"slices"
)

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

	Cloud   string `yaml:"cloud"`
	Network string `yaml:"network"`

	Serial               bool                  `yaml:"serial"`
	InstallTimeVerifiers []InstallTimeVerifier `yaml:"install_time_verifiers"` // TODO: schema?

	BaseReleasesURL         string                  `yaml:"base_releases_url"`
	Variables               []Variable              `yaml:"variables"` // TODO: schema?
	Releases                []Release               `yaml:"releases"`
	StemcellCriteria        StemcellCriteria        `yaml:"stemcell_criteria"`
	PropertyBlueprints      PropertyBlueprints      `yaml:"property_blueprints"`
	FormTypes               []FormType              `yaml:"form_types"`
	JobTypes                []JobType               `yaml:"job_types"`
	RequiresProductVersions []ProductVersion        `yaml:"requires_product_versions"`
	PostDeployErrands       []ErrandTemplate        `yaml:"post_deploy_errands"`
	PreDeleteErrands        []ErrandTemplate        `yaml:"pre_delete_errands"`
	RuntimeConfigs          []RuntimeConfigTemplate `yaml:"runtime_configs"`

	// TODO: validates_presence_of: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L20-L25
	// TODO: version_attribute: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/product_template.rb#L30-L32
	// TODO: validates_string: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/product_template.rb#L56
	// TODO: validates_integer: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/product_template.rb#L60
	// TODO: validates_manifest: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/product_template.rb#L61
	// TODO: validations: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/product_template.rb#L64-L70
	// TODO: validates: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/product_template.rb#L72
	// TODO: validates_object(s): https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/product_template.rb#L74-L82
	// TODO: find_object: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/product_template.rb#L84-L86
}

func (productTemplate *ProductTemplate) FindPropertyBlueprintWithName(name string) (PropertyBlueprint, int, error) {
	index := slices.IndexFunc(productTemplate.PropertyBlueprints, func(blueprint PropertyBlueprint) bool {
		return blueprint.PropertyName() == name
	})
	if index < 0 {
		return nil, 0, fmt.Errorf("not found")
	}
	return productTemplate.PropertyBlueprints[index], index, nil
}

func (productTemplate *ProductTemplate) HasPostDeployErrandWithName(name string) bool {
	index := slices.IndexFunc(productTemplate.PostDeployErrands, func(errandTemplate ErrandTemplate) bool {
		return errandTemplate.Name == name
	})
	return index >= 0
}

func (productTemplate *ProductTemplate) HasJobTypeWithName(name string) bool {
	index := slices.IndexFunc(productTemplate.JobTypes, func(errandTemplate JobType) bool {
		return errandTemplate.Name == name
	})
	return index >= 0
}

func (productTemplate *ProductTemplate) FindJobTypeWithName(name string) (*JobType, int, error) {
	index := slices.IndexFunc(productTemplate.JobTypes, func(errandTemplate JobType) bool {
		return errandTemplate.Name == name
	})
	if index < 0 {
		return nil, 0, fmt.Errorf("not found")
	}
	return &productTemplate.JobTypes[index], index, nil
}
