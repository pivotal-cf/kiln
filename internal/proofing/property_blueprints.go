package proofing

import yaml "gopkg.in/yaml.v2"

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
