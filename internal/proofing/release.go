package proofing

type Release struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	File    string `yaml:"file"`

	SHA1 string `yaml:"sha1"` // NOTE: this only exists because of kiln

	// TODO: validations: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/release.rb#L8-L15
}
