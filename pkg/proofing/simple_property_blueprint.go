package proofing

type SimplePropertyBlueprint struct {
	Name           string                    `yaml:"name"`
	Type           string                    `yaml:"type"`
	Default        any                       `yaml:"default"`     // TODO: schema?
	Constraints    any                       `yaml:"constraints"` // TODO: schema?
	Options        []PropertyBlueprintOption `yaml:"options"`     // TODO: schema?
	Configurable   bool                      `yaml:"configurable"`
	Optional       bool                      `yaml:"optional"`
	FreezeOnDeploy bool                      `yaml:"freeze_on_deploy"`

	Unique bool `yaml:"unique"`

	ResourceDefinitions []ResourceDefinition `yaml:"resource_definitions"`

	// TODO: validations: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/property_blueprint.rb#L27-L39
}

func (blueprint SimplePropertyBlueprint) PropertyName() string { return blueprint.Name }
func (blueprint SimplePropertyBlueprint) PropertyType() string { return blueprint.Type }
func (blueprint SimplePropertyBlueprint) HasDefault() bool     { return blueprint.Default != nil }
func (blueprint SimplePropertyBlueprint) IsConfigurable() bool { return blueprint.Configurable }

type PropertyBlueprintOption struct {
	Label string `yaml:"label"`
	Name  string `yaml:"name"`
}
