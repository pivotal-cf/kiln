package cargo

import (
	"github.com/pivotal-cf/kiln/internal/cargo/opsman"
	"github.com/pivotal-cf/kiln/internal/proofing"
	yaml "gopkg.in/yaml.v2"
)

type OpsManagerConfig struct {
	DeploymentName    string
	AvailabilityZones []string
	Stemcells         []opsman.Stemcell
	ResourceConfigs   []opsman.ResourceConfig
}

type Generator struct{}

func NewGenerator() Generator {
	return Generator{}
}

func (g Generator) Execute(template proofing.ProductTemplate, config OpsManagerConfig) Manifest {
	releases := generateReleases(template.Releases)
	stemcell := findStemcell(template.StemcellCriteria, config.Stemcells)
	update := generateUpdate(template.Serial)
	instanceGroups := generateInstanceGroups(template.JobTypes, config.ResourceConfigs, config.AvailabilityZones, stemcell.Alias)
	variables := generateVariables(template.Variables)

	return Manifest{
		Name:           config.DeploymentName,
		Releases:       releases,
		Stemcells:      []Stemcell{stemcell},
		Update:         update,
		Variables:      variables,
		InstanceGroups: instanceGroups,
	}
}

func generateReleases(templateReleases []proofing.Release) []Release {
	var releases []Release

	for _, release := range templateReleases {
		releases = append(releases, Release{
			Name:    release.Name,
			Version: release.Version,
		})
	}

	return releases
}

func findStemcell(criteria proofing.StemcellCriteria, stemcells []opsman.Stemcell) Stemcell {
	var stemcell Stemcell

	for _, s := range stemcells {
		if s.OS == criteria.OS {
			if s.Version == criteria.Version {
				stemcell = Stemcell{
					Alias:   s.Name,
					OS:      s.OS,
					Version: s.Version,
				}
			}
		}
	}

	return stemcell
}

func generateUpdate(serial bool) Update {
	return Update{
		Canaries:        1,
		CanaryWatchTime: "30000-300000",
		UpdateWatchTime: "30000-300000",
		MaxInFlight:     1,
		MaxErrors:       2,
		Serial:          serial,
	}
}

func generateInstanceGroups(jobTypes []proofing.JobType, resourceConfigs []opsman.ResourceConfig, availabilityZones []string, stemcellAlias string) []InstanceGroup {
	var instanceGroups []InstanceGroup

	for _, jobType := range jobTypes {
		lifecycle := "service"
		if jobType.Errand {
			lifecycle = "errand"
		}

		instances := jobType.InstanceDefinition.Default
		for _, resourceConfig := range resourceConfigs {
			if resourceConfig.Name == jobType.Name {
				if !resourceConfig.Instances.IsAutomatic() {
					instances = resourceConfig.Instances.Value
				}
			}
		}

		var jobs []InstanceGroupJob
		for _, template := range jobType.Templates {
			provides, err := evaluateManifestSnippet(template.Provides)
			if err != nil {
				panic(err)
			}

			consumes, err := evaluateManifestSnippet(template.Consumes)
			if err != nil {
				panic(err)
			}

			properties, err := evaluateManifestSnippet(template.Manifest)
			if err != nil {
				panic(err)
			}

			jobs = append(jobs, InstanceGroupJob{
				Name:       template.Name,
				Release:    template.Release,
				Provides:   provides,
				Consumes:   consumes,
				Properties: properties,
			})
		}

		properties, err := evaluateManifestSnippet(jobType.Manifest)
		if err != nil {
			panic(err)
		}

		instanceGroups = append(instanceGroups, InstanceGroup{
			Name:       jobType.Name,
			AZs:        availabilityZones,
			Lifecycle:  lifecycle,
			Stemcell:   stemcellAlias,
			Instances:  instances,
			Jobs:       jobs,
			Properties: properties,
		})
	}

	return instanceGroups
}

func evaluateManifestSnippet(snippet string) (interface{}, error) {
	var result interface{}

	if snippet == "" {
		snippet = "{}"
	}

	err := yaml.Unmarshal([]byte(snippet), &result)
	if err != nil {
		panic(err)
	}

	return result, nil
}

func generateVariables(templateVariables []proofing.Variable) []Variable {
	var variables []Variable

	for _, variable := range templateVariables {
		variables = append(variables, Variable{
			Name:    variable.Name,
			Options: variable.Options,
			Type:    variable.Type,
		})
	}

	return variables
}
