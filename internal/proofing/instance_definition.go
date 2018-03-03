package proofing

type InstanceDefinition struct {
	Configurable bool        `yaml:"configurable"`
	Constraints  interface{} `yaml:"constraints,omitempty"` // TODO: schema?
	Default      int         `yaml:"default"`
	Label        string      `yaml:"label"`
	Name         string      `yaml:"name"`
	Type         string      `yaml:"type"`
	ZeroIf       interface{} `yaml:"zero_if,omitempty"` // TODO: schema?
}
