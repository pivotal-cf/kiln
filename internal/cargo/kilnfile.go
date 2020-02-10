package cargo

type KilnfileLock struct {
	Releases []ReleaseLock `yaml:"releases"`
	Stemcell Stemcell      `yaml:"stemcell_criteria"`
}

type Kilnfile struct {
	ReleaseSources  []ReleaseSourceConfig `yaml:"release_sources"`
	Slug            string                `yaml:"slug"`
	PreGaUserGroups []string              `yaml:"pre_ga_user_groups"`
}

type ReleaseSourceConfig struct {
	Type            string `yaml:"type"`
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	AccessKeyId     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	PathTemplate    string `yaml:"path_template"`
	Publishable     bool   `yaml:"publishable"`
	Endpoint        string `yaml:"endpoint"`
}

type ReleaseLock struct {
	Name         string `yaml:"name"`
	SHA1         string `yaml:"sha1"`
	Version      string `yaml:"version"`
	RemoteSource string `yaml:"remote_source"`
	RemotePath   string `yaml:"remote_path"`
}
