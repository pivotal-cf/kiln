package proofing

type JobType struct {
	Name          string `yaml:"name"`
	ResourceLabel string `yaml:"resource_label"`
	Description   string `yaml:"description,omitempty"`

	MaxInFlight interface{} `yaml:"max_in_flight"`

	Serial       bool `yaml:"serial,omitempty"`
	SingleAZOnly bool `yaml:"single_az_only"`

	Templates           []Template                 `yaml:"templates"`
	InstanceDefinition  InstanceDefinition         `yaml:"instance_definition"`
	ResourceDefinitions []ResourceDefinition       `yaml:"resource_definitions"`
	PropertyBlueprints  []JobTypePropertyBlueprint `yaml:"property_blueprints,omitempty"`

	// TODO: unused?
	Label     string `yaml:"label"`
	DynamicIP int    `yaml:"dynamic_ip"`
	StaticIP  int    `yaml:"static_ip"`

	// TODO: manifest: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/job_type.rb#L8
	// TODO: max_in_flight can be int or percentage
	// TODO: canaries: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/job_type.rb#L17
	// TODO: errand: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/job_type.rb#L21
	// TODO: run_pre_delete_errand_default: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/job_type.rb#L22
	// TODO: run_post_deploy_errand_default: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/job_type.rb#L23
}

type JobTypePropertyBlueprint struct {
	Configurable bool        `yaml:"configurable,omitempty"`
	Constraints  interface{} `yaml:"constraints,omitempty"` // TODO: schema
	Default      interface{} `yaml:"default,omitempty"`     // TODO: schema?
	Label        string      `yaml:"label,omitempty"`
	Name         string      `yaml:"name"`
	Optional     bool        `yaml:"optional,omitempty"`
	Type         string      `yaml:"type"`
}
