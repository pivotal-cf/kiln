package proofing

type StemcellCriteria struct {
	OS      string `yaml:"os"`
	Version string `yaml:"version"`

	// TODO: enable_patch_security_updates: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/stemcell_criteria.rb#L6
}
