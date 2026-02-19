package models

type RuntimeConfigOuter struct {
	Name          string `yaml:"name"`
	RuntimeConfig string `yaml:"runtime_config"`
}

type RuntimeConfigInner struct {
	Releases []string `yaml:"releases"`
	Addons   []Addon  `yaml:"addons"`
}

type Addon struct {
	Name    string    `yaml:"name"`
	Include Inclusion `yaml:"include"`
	Jobs    []Job     `yaml:"jobs"`
}

type Inclusion struct {
	Deployments []string `yaml:"deployments"`
	Jobs        []Job    `yaml:"jobs"`
}
