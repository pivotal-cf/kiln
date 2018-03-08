package proofing

import (
	yaml "gopkg.in/yaml.v2"
)

type PropertyBlueprint interface{}

type PropertyBlueprints []PropertyBlueprint

// TODO: Less ugly.
func (pb *PropertyBlueprints) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	var sniffs []map[string]interface{}
	err := unmarshal(&sniffs)
	if err != nil {
		panic(err)
	}

	for _, sniff := range sniffs {
		contents, err := yaml.Marshal(sniff)
		if err != nil {
			panic(err)
		}

		switch sniff["type"] {
		case "selector":
			var propertyBlueprint SelectorPropertyBlueprint
			err = yaml.Unmarshal(contents, &propertyBlueprint)
			*pb = append(*pb, propertyBlueprint)
		case "collection":
			var propertyBlueprint CollectionPropertyBlueprint
			err = yaml.Unmarshal(contents, &propertyBlueprint)
			*pb = append(*pb, propertyBlueprint)
		default:
			var propertyBlueprint SimplePropertyBlueprint
			err = yaml.Unmarshal(contents, &propertyBlueprint)
			*pb = append(*pb, propertyBlueprint)
		}
		if err != nil {
			panic(err)
		}
	}

	return nil
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
