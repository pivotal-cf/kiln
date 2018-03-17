package proofing

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

type PropertyBlueprintPropertyBlueprint struct {
	Configurable bool        `yaml:"configurable"`
	Default      interface{} `yaml:"default"` // TODO: how to validate?
	Name         string      `yaml:"name"`
	Type         string      `yaml:"type"`
}
