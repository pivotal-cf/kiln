package models

type Job struct {
	Name       string                         `yaml:"name"`
	Release    string                         `yaml:"release"`
	Properties map[string]PackageInstallProps `yaml:"properties,omitempty"`
}
