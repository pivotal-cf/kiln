package proofing

type SelectorPropertyBlueprint struct {
	SimplePropertyBlueprint `yaml:",inline"`

	NamedManifests     []PropertyBlueprintNamedManifest     `yaml:"named_manifests"`
	OptionTemplates    []PropertyBlueprintOptionTemplate    `yaml:"option_templates"`
	PropertyBlueprints []PropertyBlueprintPropertyBlueprint `yaml:"property_blueprints"`
}
