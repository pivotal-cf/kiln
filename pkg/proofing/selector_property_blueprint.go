package proofing

type SelectorPropertyBlueprint struct {
	SimplePropertyBlueprint `yaml:",inline"`

	OptionTemplates []SelectorPropertyOptionTemplate `yaml:"option_templates"`

	// TODO: validations: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/selector_property_blueprint.rb#L10
	// TODO: find_object: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/selector_property_blueprint.rb#L8
}
