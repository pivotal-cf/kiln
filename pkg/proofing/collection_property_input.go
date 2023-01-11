package proofing

type CollectionPropertyInput struct {
	SimplePropertyInput `yaml:",inline"`

	PropertyInputs []CollectionSubfieldPropertyInput `yaml:"property_inputs"`
}

func (input CollectionPropertyInput) Ref() string {
	return input.SimplePropertyInput.Reference
}
