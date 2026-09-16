package models

// DownwardAPIItem represents a single entry under spec.template.valuesFrom.downwardAPI.items
// in the generated kapp-controller PackageInstall resource. Currently only kubernetesAPIs
// is supported; additional item types (fieldRef, resourceFieldRef) can be added later.
type DownwardAPIItem struct {
	Name           string    `yaml:"name"`
	KubernetesAPIs *struct{} `yaml:"kubernetesAPIs,omitempty"`
}

type PackageInstall struct {
	Name             string            `yaml:"name"`
	PackageName      string            `yaml:"packageName"`
	PackageVersion   string            `yaml:"packageVersion"`
	Values           interface{}       `yaml:"values,omitempty"`
	DownwardAPIItems []DownwardAPIItem `yaml:"downwardAPIItems,omitempty"`
}
