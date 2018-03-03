package proofing

// TODO: property_blueprints can be of differing types: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/property_blueprint_builder.rb
type PropertyBlueprint struct {
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

type PropertyBlueprintNamedManifest struct {
	Manifest string `yaml:"manifest"`
	Name     string `yaml:"name"`
}

type PropertyBlueprintOptionTemplate struct {
	Name               string                                             `yaml:"name"`
	NamedManifests     []PropertyBlueprintOptionTemplateNamedManifest     `yaml:"named_manifests"`
	PropertyBlueprints []PropertyBlueprintOptionTemplatePropertyBlueprint `yaml:"property_blueprints"`
	SelectValue        string                                             `yaml:"select_value"`
}

type PropertyBlueprintOptionTemplateNamedManifest struct {
	Manifest string `yaml:"manifest"`
	Name     string `yaml:"name"`
}

type PropertyBlueprintOptionTemplatePropertyBlueprint struct {
	Configurable bool                                                     `yaml:"configurable"`
	Constraints  interface{}                                              `yaml:"constraints"` //TODO: schema?
	Default      interface{}                                              `yaml:"default"`     // TODO: schema?
	Name         string                                                   `yaml:"name"`
	Optional     bool                                                     `yaml:"optional"`
	Options      []PropertyBlueprintOptionTemplatePropertyBlueprintOption `yaml:"options"`
	Placeholder  string                                                   `yaml:"placeholder"`
	Type         string                                                   `yaml:"type"`
}

type PropertyBlueprintOptionTemplatePropertyBlueprintOption struct {
	Label string `yaml:"label"`
	Name  string `yaml:"name"`
}

type PropertyBlueprintOption struct {
	Label string `yaml:"label"`
	Name  string `yaml:"name"`
}

type PropertyBlueprintPropertyBlueprint struct {
	Configurable bool        `yaml:"configurable"`
	Default      interface{} `yaml:"default"` // TODO: how to validate?
	Name         string      `yaml:"name"`
	Type         string      `yaml:"type"`
}
