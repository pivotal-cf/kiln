package proofing

type Release struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	File    string `yaml:"file"`

	SHA1 string `yaml:"sha1"` // NOTE: this only exists because of kiln
}
