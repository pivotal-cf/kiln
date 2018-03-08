package proofing

// TODO: property_blueprints can be of differing types: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/property_blueprint_builder.rb
type SimplePropertyBlueprint struct {
	Configurable       bool                                 `yaml:"configurable"`
	Default            interface{}                          `yaml:"default"` // TODO: schema?
	Name               string                               `yaml:"name"`
	NamedManifests     []PropertyBlueprintNamedManifest     `yaml:"named_manifests"`
	OptionTemplates    []PropertyBlueprintOptionTemplate    `yaml:"option_templates"`
	Optional           bool                                 `yaml:"optional"`
	Options            []PropertyBlueprintOption            `yaml:"options"`
	PropertyBlueprints []PropertyBlueprintPropertyBlueprint `yaml:"property_blueprints"`
	Type               string                               `yaml:"type"`
}
