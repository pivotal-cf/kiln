package proofing

import (
	"fmt"

	"github.com/crhntr/yamlutil/yamlnode"
	"gopkg.in/yaml.v3"
)

type PropertyBlueprint interface {
	PropertyName() string
	PropertyType() string
	HasDefault() bool
	IsConfigurable() bool
}

type PropertyBlueprints []PropertyBlueprint

func (pb *PropertyBlueprints) UnmarshalYAML(list *yaml.Node) error {
	if list.Kind != yaml.SequenceNode {
		return fmt.Errorf("expected a list of property inputs")
	}
	for _, element := range list.Content {
		propertyBlueprint, err := unmarshalPropertyBlueprint(element)
		if err != nil {
			return err
		}
		*pb = append(*pb, propertyBlueprint)
	}
	return nil
}

func unmarshalPropertyBlueprint(node *yaml.Node) (PropertyBlueprint, error) {
	typeField, found := yamlnode.LookupKey(node, "type")
	if found {
		switch typeField.Value {
		case "selector":
			var selectorPropertyInputs SelectorPropertyBlueprint
			err := node.Decode(&selectorPropertyInputs)
			if err != nil {
				return nil, err
			}
			return &selectorPropertyInputs, nil
		case "collection":
			var collectionPropertyInput CollectionPropertyBlueprint
			err := node.Decode(&collectionPropertyInput)
			if err != nil {
				return nil, err
			}
			return &collectionPropertyInput, nil
		}
	}
	var simplePropertyBlueprint SimplePropertyBlueprint
	err := node.Decode(&simplePropertyBlueprint)
	if err != nil {
		return nil, err
	}
	return &simplePropertyBlueprint, nil
}
