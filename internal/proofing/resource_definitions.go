package proofing

// TODO: https://github.com/pivotal-cf/installation/blob/b7be08d7b50d305c08d520ee0afe81ae3a98bd9d/web/app/models/persistence/metadata/resource_definition_collection.rb
type ResourceDefinitions []ResourceDefinition

type ResourceDefinition struct {
	Configurable bool        `yaml:"configurable"`
	Constraints  interface{} `yaml:"constraints,omitempty"` // TODO: schema?
	Default      int         `yaml:"default"`
	Label        string      `yaml:"label"`
	Name         string      `yaml:"name"`
	Type         string      `yaml:"type"`
}
