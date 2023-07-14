package proofing

type InstallTimeVerifier struct {
	Ignorable  bool   `yaml:"ignorable,omitempty"`
	Name       string `yaml:"name"`
	Properties any    `yaml:"properties"` // TODO: schema?
}
