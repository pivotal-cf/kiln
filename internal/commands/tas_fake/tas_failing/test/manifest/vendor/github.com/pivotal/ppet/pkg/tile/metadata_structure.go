package tile

// Metadata represents the tile file metadata/metadata.yml
// Documentation on the fields can be found here: https://docs.pivotal.io/tiledev/2-10/property-template-references.html
type Metadata struct {
	Name               string              `yaml:"name"`
	ProductVersion     string              `yaml:"product_version"`
	PropertyBlueprints []PropertyBlueprint `yaml:"property_blueprints"`
	PostDeployErrands  []ColocatedErrand   `yaml:"post_deploy_errands"`
	JobTypes           []JobType           `yaml:"job_types"`
}

func (m Metadata) FindPropertyBlueprintWithName(name string) (PropertyBlueprint, int, bool) {
	for index, bp := range m.PropertyBlueprints {
		if bp.Name == name {
			return bp, index, true
		}
	}
	return PropertyBlueprint{}, len(m.PropertyBlueprints), false
}

func (m Metadata) HasPostDeployErrandWithName(name string) bool {
	for _, e := range m.PostDeployErrands {
		if e.Name == name {
			return true
		}
	}
	return false
}

func (m Metadata) HasJobTypeWithName(name string) bool {
	for _, j := range m.JobTypes {
		if j.Name == name {
			return true
		}
	}
	return false
}

func (m Metadata) FindJobTypeWithName(name string) (JobType, bool) {
	for _, j := range m.JobTypes {
		if j.Name == name {
			return j, true
		}
	}
	return JobType{}, false
}

type PropertyBlueprint struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`

	IsConfigurable bool `yaml:"configurable"`

	Default any `yaml:"default,omitempty"`
}

func (pb PropertyBlueprint) HasDefault() bool {
	return pb.Default != nil
}

// ColocatedErrand only contains the fields we use. For the rest of the fields, see https://docs.pivotal.io/tiledev/2-10/tile-errands.html#define-a-colocated-errand
type ColocatedErrand struct {
	Name string `yaml:"name"`
}

// A JobType defines the jobs that end up in a BOSH manifest.
// See more here: https://docs.pivotal.io/tiledev/2-10/property-template-references.html#job-types
type JobType struct {
	Name               string             `yaml:"name"`
	InstanceDefinition InstanceDefinition `yaml:"instance_definition"`
}

// An InstanceDefinition is described here: https://docs.pivotal.io/tiledev/2-10/property-template-references.html#job-instance-def
// However, the hash properties are not clearly defined. So here is the source code:
// https://github.com/pivotal-cf/ops-manager/blob/8ea95ba9b33ffbf45870c9069735780f2855e500/web/app/models/persistence/metadata/resource_definition.rb#L3
type InstanceDefinition struct {
	Configurable bool        `yaml:"configurable"`
	Default      int         `yaml:"default"`
	Constraints  Constraints `yaml:"constraints"`
}

type Constraints struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}
