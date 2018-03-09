package proofing

// TODO: property_blueprints can be of differing types: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/property_blueprint_builder.rb
type SimplePropertyBlueprint struct {
	Name                string                    `yaml:"name"`
	Type                string                    `yaml:"type"`
	Default             interface{}               `yaml:"default"` // TODO: schema?
	Constraints         Constraint                `yaml:"constraints"`
	Options             []PropertyBlueprintOption `yaml:"options"`
	Configurable        bool                      `yaml:"configurable"`
	Optional            bool                      `yaml:"optional"`
	FreezeOnDeploy      bool                      `yaml:"freeze_on_deploy"`
	Unique              bool                      `yaml:"unique"`
	ResourceDefinitions []ResourceDefinition      `yaml:"resource_definitions"`
}
