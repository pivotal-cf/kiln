package models

type PackageInstall struct {
	Name           string      `yaml:"name"`
	PackageName    string      `yaml:"packageName"`
	PackageVersion string      `yaml:"packageVersion"`
	Values         interface{} `yaml:"values,omitempty"`
}
