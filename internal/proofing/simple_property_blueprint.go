package proofing

type SimplePropertyBlueprint struct {
	Name                string                    `yaml:"name"`
	Type                string                    `yaml:"type"`
	Default             interface{}               `yaml:"default"` // TODO: schema?
	Constraints         Constraint                `yaml:"constraints"`
	Options             []PropertyBlueprintOption `yaml:"options"`
	Configurable        bool                      `yaml:"configurable"`
	Optional            bool                      `yaml:"optional"`
	FreezeOnDeploy      bool                      `yaml:"freeze_on_deploy"`
	Unique              bool                      `yaml:"unique"`
	ResourceDefinitions []ResourceDefinition      `yaml:"resource_definitions"`
}
