package proofing

import (
	"fmt"

	"github.com/crhntr/yamlutil/yamlnode"
	"gopkg.in/yaml.v3"
)

type PropertyInput interface {
	Ref() string
}

type PropertyInputs []PropertyInput

func (pi *PropertyInputs) UnmarshalYAML(list *yaml.Node) error {
	if list.Kind != yaml.SequenceNode {
		return fmt.Errorf("expected a list of property inputs")
	}
	for _, property := range list.Content {
		if _, hasSelectorPropertyInputsField := yamlnode.LookupKey(property, "selector_property_inputs"); hasSelectorPropertyInputsField {
			var selectorPropertyInputs SelectorPropertyInput
			err := property.Decode(&selectorPropertyInputs)
			if err != nil {
				return err
			}
			*pi = append(*pi, selectorPropertyInputs)
		} else if _, hasPropertyInputsField := yamlnode.LookupKey(property, "property_inputs"); hasPropertyInputsField {
			var collectionPropertyInput CollectionPropertyInput
			err := property.Decode(&collectionPropertyInput)
			if err != nil {
				return err
			}
			*pi = append(*pi, collectionPropertyInput)
			continue
		} else {
			var propertyInput SimplePropertyInput
			err := property.Decode(&propertyInput)
			if err != nil {
				return err
			}
			*pi = append(*pi, propertyInput)
		}
	}
	return nil
}
