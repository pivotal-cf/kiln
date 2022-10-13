package proofing

type NamedManifest struct {
	Name     string `yaml:"name"`
	Manifest string `yaml:"manifest"`

	// TODO: validations: https://github.com/pivotal-cf/installation/blob/039a2ef3f751ef5915c425da8150a29af4b764dd/web/app/models/persistence/metadata/named_manifest.rb#L7
}
