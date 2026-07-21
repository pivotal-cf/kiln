package models

type Job struct {
	Name       string                         `yaml:"name"`
	Release    string                         `yaml:"release"`
	Consumes   map[string]JobConsumes         `yaml:"consumes,omitempty"`
	Properties map[string]PackageInstallProps `yaml:"properties,omitempty"`
}

// JobConsumes declares cross-deployment link resolution for a runtime config addon job.
// When a job-spec-overlay declares runtime_config_from or runtime_config_deployment,
// kiln includes this entry in the addon job's consumes map in the generated metadata.
type JobConsumes struct {
	From       string `yaml:"from,omitempty"`
	Deployment string `yaml:"deployment,omitempty"`
}
