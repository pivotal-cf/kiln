package models

type PackageInstallProps struct {
	Name    string      `yaml:"name"`
	Version string      `yaml:"version"`
	Values  interface{} `yaml:"values,omitempty"`
}
