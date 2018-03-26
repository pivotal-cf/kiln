package cargo

type Manifest struct {
	Name      string     `yaml:"name"`
	Releases  []Release  `yaml:"releases"`
	Stemcells []Stemcell `yaml:"stemcells"`
	Update    Update     `yaml:"update"`
}

type Release struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Stemcell struct {
	Alias   string `yaml:"alias"`
	OS      string `yaml:"os"`
	Version string `yaml:"version"`
}

type Update struct {
	Canaries        int    `yaml:"canaries"`
	CanaryWatchTime string `yaml:"canary_watch_time"`
	UpdateWatchTime string `yaml:"update_watch_time"`
	MaxInFlight     int    `yaml:"max_in_flight"`
	MaxErrors       int    `yaml:"max_errors"`
	Serial          bool   `yaml:"serial"`
}
