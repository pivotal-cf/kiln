package builder

import (
	"fmt"
	"path/filepath"
)

type MetadataBuilder struct {
	formDirectoryReader           formDirectoryReader
	instanceGroupDirectoryReader  instanceGroupDirectoryReader
	jobsDirectoryReader           jobsDirectoryReader
	iconEncoder                   iconEncoder
	logger                        logger
	metadataReader                metadataReader
	releaseManifestReader         releaseManifestReader
	runtimeConfigsDirectoryReader metadataPartsDirectoryReader
	stemcellManifestReader        stemcellManifestReader
	variablesDirectoryReader      metadataPartsDirectoryReader
}

type BuildInput struct {
	MetadataPath             string
	ReleaseTarballs          []string
	StemcellTarball          string
	FormDirectories          []string
	InstanceGroupDirectories []string
	JobDirectories           []string
	RuntimeConfigDirectories []string
	VariableDirectories      []string
	IconPath                 string
	Version                  string
}

type GeneratedMetadata struct {
	FormTypes        []interface{} `yaml:"form_types,omitempty"`
	JobTypes         []interface{} `yaml:"job_types,omitempty"`
	IconImage        string        `yaml:"icon_image"`
	Metadata         Metadata      `yaml:",inline"`
	Name             string
	Releases         []Release
	RuntimeConfigs   []interface{}    `yaml:"runtime_configs,omitempty"`
	StemcellCriteria StemcellCriteria `yaml:"stemcell_criteria"`
	Variables        []interface{}    `yaml:",omitempty"`
}

type Release struct {
	Name    string
	File    string
	Version string
}

type StemcellCriteria struct {
	Version     string
	OS          string
	RequiresCPI bool `yaml:"requires_cpi"`
}

type BoshRuntimeConfigFields map[string]interface{}

type BoshRuntimeConfig struct {
	Releases    []map[string]string     `yaml:",omitempty"`
	OtherFields BoshRuntimeConfigFields `yaml:",inline"`
}

//go:generate counterfeiter -o ./fakes/release_manifest_reader.go --fake-name ReleaseManifestReader . releaseManifestReader

type releaseManifestReader interface {
	Read(path string) (ReleaseManifest, error)
}

//go:generate counterfeiter -o ./fakes/stemcell_manifest_reader.go --fake-name StemcellManifestReader . stemcellManifestReader

type stemcellManifestReader interface {
	Read(path string) (StemcellManifest, error)
}

//go:generate counterfeiter -o ./fakes/metadata_parts_directory_reader.go --fake-name MetadataPartsDirectoryReader . metadataPartsDirectoryReader

type metadataPartsDirectoryReader interface {
	Read(path string) ([]interface{}, error)
}

//go:generate counterfeiter -o ./fakes/form_directory_reader.go --fake-name FormDirectoryReader . formDirectoryReader

type formDirectoryReader interface {
	Read(path string) ([]interface{}, error)
}

//go:generate counterfeiter -o ./fakes/instance_group_directory_reader.go --fake-name InstanceGroupDirectoryReader . instanceGroupDirectoryReader

type instanceGroupDirectoryReader interface {
	Read(path string) ([]interface{}, error)
}

//go:generate counterfeiter -o ./fakes/jobs_directory_reader.go --fake-name JobsDirectoryReader . jobsDirectoryReader

type jobsDirectoryReader interface {
	Read(path string) ([]interface{}, error)
}

//go:generate counterfeiter -o ./fakes/metadata_reader.go --fake-name MetadataReader . metadataReader

type metadataReader interface {
	Read(path, version string) (Metadata, error)
}

//go:generate counterfeiter -o ./fakes/icon_encoder.go --fake-name IconEncoder . iconEncoder

type iconEncoder interface {
	Encode(path string) (string, error)
}

type logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

func NewMetadataBuilder(
	formDirectoryReader formDirectoryReader,
	instanceGroupDirectoryReader instanceGroupDirectoryReader,
	jobsDirectoryReader jobsDirectoryReader,
	releaseManifestReader releaseManifestReader,
	runtimeConfigsDirectoryReader,
	variablesDirectoryReader metadataPartsDirectoryReader,
	stemcellManifestReader stemcellManifestReader,
	metadataReader metadataReader,
	logger logger,
	iconEncoder iconEncoder,
) MetadataBuilder {
	return MetadataBuilder{
		formDirectoryReader:           formDirectoryReader,
		instanceGroupDirectoryReader:  instanceGroupDirectoryReader,
		jobsDirectoryReader:           jobsDirectoryReader,
		iconEncoder:                   iconEncoder,
		logger:                        logger,
		metadataReader:                metadataReader,
		releaseManifestReader:         releaseManifestReader,
		runtimeConfigsDirectoryReader: runtimeConfigsDirectoryReader,
		stemcellManifestReader:        stemcellManifestReader,
		variablesDirectoryReader:      variablesDirectoryReader,
	}
}

func (m MetadataBuilder) Build(input BuildInput) (GeneratedMetadata, error) {
	var releases []Release
	for _, releaseTarball := range input.ReleaseTarballs {
		releaseManifest, err := m.releaseManifestReader.Read(releaseTarball)
		if err != nil {
			return GeneratedMetadata{}, err
		}

		m.logger.Printf("Read manifest for release %s", releaseManifest.Name)

		releases = append(releases, Release{
			Name:    releaseManifest.Name,
			Version: releaseManifest.Version,
			File:    filepath.Base(releaseTarball),
		})
	}

	var runtimeConfigs []interface{}
	for _, runtimeConfigsDirectory := range input.RuntimeConfigDirectories {
		r, err := m.runtimeConfigsDirectoryReader.Read(runtimeConfigsDirectory)
		if err != nil {
			return GeneratedMetadata{},
				fmt.Errorf("error reading from runtime configs directory %q: %s", runtimeConfigsDirectory, err)
		}

		m.logger.Printf("Read runtime configs from %s", runtimeConfigsDirectory)

		runtimeConfigs = append(runtimeConfigs, r...)
	}

	var variables []interface{}
	for _, variablesDirectory := range input.VariableDirectories {
		v, err := m.variablesDirectoryReader.Read(variablesDirectory)
		if err != nil {
			return GeneratedMetadata{},
				fmt.Errorf("error reading from variables directory %q: %s", variablesDirectory, err)
		}

		m.logger.Printf("Read variables from %s", variablesDirectory)

		variables = append(variables, v...)
	}

	stemcellManifest, err := m.stemcellManifestReader.Read(input.StemcellTarball)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	m.logger.Printf("Read manifest for stemcell version %s", stemcellManifest.Version)

	encodedIcon, err := m.iconEncoder.Encode(input.IconPath)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	metadata, err := m.metadataReader.Read(input.MetadataPath, input.Version)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	productName, ok := metadata["name"].(string)
	if !ok {
		return GeneratedMetadata{}, fmt.Errorf(`missing "name" in tile metadata file '%s'`, input.MetadataPath)
	}

	var formTypes []interface{}
	if len(input.FormDirectories) > 0 {
		for _, fd := range input.FormDirectories {
			m.logger.Printf("Read forms from %s", fd)
			formTypesInDir, err := m.formDirectoryReader.Read(fd)
			if err != nil {
				return GeneratedMetadata{},
					fmt.Errorf("error reading from form directory %q: %s", fd, err)
			}
			formTypes = append(formTypes, formTypesInDir...)
		}
	} else {
		if ft, ok := metadata["form_types"].([]interface{}); ok {
			formTypes = ft
		}
	}

	var jobTypes []interface{}
	if len(input.InstanceGroupDirectories) > 0 {
		for _, jd := range input.InstanceGroupDirectories {
			m.logger.Printf("Read instance groups from %s", jd)
			jobTypesInDir, err := m.instanceGroupDirectoryReader.Read(jd)
			if err != nil {
				return GeneratedMetadata{},
					fmt.Errorf("error reading from instance group directory %q: %s", jd, err)
			}
			jobTypes = append(jobTypes, jobTypesInDir...)
		}
	} else {
		if jt, ok := metadata["job_types"].([]interface{}); ok {
			jobTypes = jt
		}
	}

	jobs := map[interface{}]interface{}{}
	for _, jobDir := range input.JobDirectories {
		m.logger.Printf("Read jobs from %s", jobDir)
		jobsInDir, err := m.jobsDirectoryReader.Read(jobDir)
		if err != nil {
			return GeneratedMetadata{},
				fmt.Errorf("error reading from job directory %q: %s", jobDir, err)
		}
		for _, j := range jobsInDir {
			if name, ok := j.(map[interface{}]interface{})["name"]; ok {
				jobs[name] = j
			}
		}
	}

	for _, jt := range jobTypes {
		if templates, ok := jt.(map[interface{}]interface{})["templates"]; ok {
			for i, template := range templates.([]interface{}) {
				if job, ok := jobs[template]; ok {
					templates.([]interface{})[i] = job
				}
			}
		}
	}

	delete(metadata, "name")
	delete(metadata, "icon_image")
	delete(metadata, "form_types")
	delete(metadata, "job_types")

	if _, present := metadata["runtime_configs"]; present {
		return GeneratedMetadata{}, fmt.Errorf(
			"runtime_config section must be defined using --runtime-configs-directory flag, not in %q", input.MetadataPath)
	}

	if _, present := metadata["variables"]; present {
		return GeneratedMetadata{}, fmt.Errorf(
			"variables section must be defined using --variables-directory flag, not in %q", input.MetadataPath)
	}

	m.logger.Printf("Read metadata")

	return GeneratedMetadata{
		FormTypes:      formTypes,
		JobTypes:       jobTypes,
		IconImage:      encodedIcon,
		Name:           productName,
		Releases:       releases,
		RuntimeConfigs: runtimeConfigs,
		Variables:      variables,
		StemcellCriteria: StemcellCriteria{
			OS:          stemcellManifest.OperatingSystem,
			Version:     stemcellManifest.Version,
			RequiresCPI: false,
		},
		Metadata: metadata,
	}, nil
}
