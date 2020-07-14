package cargo

type KilnfileLock struct {
	Releases []ReleaseLock `yaml:"releases"`
	Stemcell Stemcell      `yaml:"stemcell_criteria"`
}

type ReleaseKiln struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	Stemcell string `yaml:"stemcell"`
}

type Kilnfile struct {
	ReleaseSources  []ReleaseSourceConfig `yaml:"release_sources"`
	Slug            string                `yaml:"slug"`
	PreGaUserGroups []string              `yaml:"pre_ga_user_groups"`
	Releases        []ReleaseKiln         `yaml:"releases"`
}

type ReleaseSourceConfig struct {
	Type            string `yaml:"type"`
	ID              string `yaml:"id"`
	Publishable     bool   `yaml:"publishable"`
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	AccessKeyId     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	PathTemplate    string `yaml:"path_template"`
	Endpoint        string `yaml:"endpoint"`
}

type ReleaseLock struct {
	Name         string `yaml:"name"`
	SHA1         string `yaml:"sha1"`
	Version      string `yaml:"version"`
	RemoteSource string `yaml:"remote_source"`
	RemotePath   string `yaml:"remote_path"`
}
