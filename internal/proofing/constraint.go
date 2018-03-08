package proofing

type Constraint struct {
	Min            int    `yaml:"min"`
	Max            int    `yaml:"max"`
	MustMatchRegex string `yaml:"must_match_regex"`
	ErrorMessage   string `yaml:"error_message"`
}
