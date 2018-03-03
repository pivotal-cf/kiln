package proofing

type ErrandTemplate struct {
	Colocated   bool     `yaml:"colocated"`
	Description string   `yaml:"description"`
	Instances   []string `yaml:"instances"` // TODO: how to validate?
	Label       string   `yaml:"label"`
	Name        string   `yaml:"name"`
	RunDefault  bool     `yaml:"run_default"`
}
