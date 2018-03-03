package proofing

type FormType struct {
	PropertyInputs []FormTypePropertyInput `yaml:"property_inputs"`
	Verifiers      []VerifierBlueprint     `yaml:"verifiers,omitempty"`

	Name        string `yaml:"name"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
	Markdown    string `yaml:"markdown,omitempty"`

	// TODO: property_inputs can be different types: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/property_input_builder.rb
}

type FormTypePropertyInput struct {
	Description            string                                       `yaml:"description,omitempty"`
	Label                  string                                       `yaml:"label"`
	Placeholder            string                                       `yaml:"placeholder,omitempty"`
	PropertyInputs         []FormTypePropertyInputPropertyInput         `yaml:"property_inputs,omitempty"`
	Reference              string                                       `yaml:"reference"`
	SelectorPropertyInputs []FormTypePropertyInputSelectorPropertyInput `yaml:"selector_property_inputs,omitempty"`
}

type FormTypePropertyInputPropertyInput struct {
	Description string `yaml:"description,omitempty"`
	Label       string `yaml:"label"`
	Reference   string `yaml:"reference"`
}

type FormTypePropertyInputSelectorPropertyInput struct {
	Description    string                                                    `yaml:"description,omitempty"`
	Label          string                                                    `yaml:"label"`
	PropertyInputs []FormTypePropertyInputSelectorPropertyInputPropertyInput `yaml:"property_inputs,omitempty"`
	Reference      string                                                    `yaml:"reference"`
}

type FormTypePropertyInputSelectorPropertyInputPropertyInput struct {
	Description string `yaml:"description,omitempty"`
	Label       string `yaml:"label"`
	Placeholder string `yaml:"placeholder,omitempty"`
	Reference   string `yaml:"reference"`
}
