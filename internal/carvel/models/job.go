package models

type Job struct {
	Name       string                 `yaml:"name"`
	Release    string                 `yaml:"release"`
	Consumes   map[string]JobConsumes `yaml:"consumes,omitempty"`
	Properties map[string]interface{} `yaml:"properties,omitempty"`
}

type JobConsumes struct {
	From       string `yaml:"from,omitempty"`
	Deployment string `yaml:"deployment,omitempty"`
}
