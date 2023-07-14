package proofing

type Variable struct {
	Name    string `yaml:"name"`
	Options any    `yaml:"options,omitempty"` // TODO: schema?
	Type    string `yaml:"type"`
}
