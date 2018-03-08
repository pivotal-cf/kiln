package proofing

type CollectionPropertyBlueprint struct {
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
