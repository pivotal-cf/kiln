package cargo

type S3ReleaseSource struct {
	_ struct{} `yaml:"-"`

	Publishable bool   `yaml:"publishable"`
	Identifier  string `yaml:"id"`

	Bucket       string `yaml:"bucket"`
	Region       string `yaml:"region"`
	PathTemplate string `yaml:"path_template"`
	Endpoint     string `yaml:"endpoint"`

	// secrets

	AccessKeyId     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
}

func (rs S3ReleaseSource) Type() string { return ReleaseSourceTypeS3 }

func (rs S3ReleaseSource) ID() string {
	if rs.Identifier != "" {
		return rs.Identifier
	}
	return rs.Bucket
}

func (rs S3ReleaseSource) IsPublishable() bool {
	return rs.Publishable
}

func (rs S3ReleaseSource) ConfigureSecrets(tv TemplateVariables) (ReleaseSource, error) {
	var err error
	rs.AccessKeyId, err = configureSecret(rs.AccessKeyId, "access_key_id", "AWS_ACCESS_KEY_ID", tv)
	if err != nil {
		return rs, err
	}
	rs.SecretAccessKey, err = configureSecret(rs.SecretAccessKey, "secret_access_key", "AWS_SECRET_ACCESS_KEY", tv)
	if err != nil {
		return rs, err
	}
	return rs, nil
}
