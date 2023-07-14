package proofing

type VerifierBlueprint struct {
	Name       string `yaml:"name"`
	Properties any    `yaml:"properties"` // TODO: schema?
}
