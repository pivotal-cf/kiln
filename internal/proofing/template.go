package proofing

type Template struct {
	Consumes string `yaml:"consumes,omitempty"`
	Manifest string `yaml:"manifest,omitempty"`
	Name     string `yaml:"name"`
	Release  string `yaml:"release"`
	Provides string `yaml:"provides,omitempty"`
}
