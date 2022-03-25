package cargo

type S3ReleaseSource struct {
	_ struct{} `yaml:"-"`

	Publishable bool   `yaml:"publishable"`
	Identifier  string `yaml:"id"`

	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	AccessKeyId     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	PathTemplate    string `yaml:"path_template"`
	Endpoint        string `yaml:"endpoint"`
}

func (s3rs S3ReleaseSource) Type() string { return ReleaseSourceTypeS3 }

func (s3rs S3ReleaseSource) ID() string {
	if s3rs.Identifier != "" {
		return s3rs.Identifier
	}
	return s3rs.Bucket
}

func (s3rs S3ReleaseSource) IsPublishable() bool {
	return s3rs.Publishable
}
