package models

type Job struct {
	Name    string `yaml:"name"`
	Release string `yaml:"release"`
	// Properties is intentionally map[string]interface{} so both the
	// PackageInstall-shaped registry-data job and simpler additional jobs
	// (e.g. script-deposit jobs with no PackageInstall-shaped payload) can
	// coexist without being forced into the {name, version, values} schema.
	Properties map[string]interface{} `yaml:"properties,omitempty"`
}
