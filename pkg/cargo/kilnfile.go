package cargo

import "errors"

type KilnfileLock struct {
	Releases []ComponentLock `yaml:"releases"`
	Stemcell Stemcell        `yaml:"stemcell_criteria"`
}

type ComponentSpec struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Kilnfile struct {
	ReleaseSources  []ReleaseSourceConfig `yaml:"release_sources"`
	Slug            string                `yaml:"slug"`
	PreGaUserGroups []string              `yaml:"pre_ga_user_groups"`
	Releases        []ComponentSpec       `yaml:"releases"`
	TileNames       []string              `yaml:"tile_names"`
	Stemcell        Stemcell              `yaml:"stemcell_criteria"`
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

type ComponentLock struct {
	ComponentSpec `yaml:",inline"`

	SHA1         string `yaml:"sha1"`
	RemoteSource string `yaml:"remote_source"`
	RemotePath   string `yaml:"remote_path"`
}

func (k KilnfileLock) FindReleaseWithName(name string) (ComponentLock, error) {
	for _, r := range k.Releases {
		if r.Name == name {
			return r, nil
		}
	}
	return ComponentLock{}, errors.New("not found")
}
