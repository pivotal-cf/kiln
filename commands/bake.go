package commands

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
)

//go:generate counterfeiter -o ./fakes/interpolator.go --fake-name Interpolator . interpolator
type interpolator interface {
	Interpolate(input builder.InterpolateInput, templateYAML []byte) ([]byte, error)
}

//go:generate counterfeiter -o ./fakes/tile_writer.go --fake-name TileWriter . tileWriter
type tileWriter interface {
	Write(generatedMetadataContents []byte, input builder.WriteInput) error
}

//go:generate counterfeiter -o ./fakes/metadata_builder.go --fake-name MetadataBuilder . metadataBuilder
type metadataBuilder interface {
	Build(input builder.BuildInput) (builder.GeneratedMetadata, error)
}

//go:generate counterfeiter -o ./fakes/directory_reader.go --fake-name DirectoryReader . directoryReader
type directoryReader interface {
	Read(path string) ([]builder.Part, error)
}

//go:generate counterfeiter -o ./fakes/logger.go --fake-name Logger . logger
type logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

//go:generate counterfeiter -o ./fakes/template_variables_service.go --fake-name TemplateVariablesService . templateVariablesService
type templateVariablesService interface {
	FromPathsAndPairs(paths []string, pairs []string) (variables map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/releases_service.go --fake-name ReleasesService . releasesService
type releasesService interface {
	FromDirectories(directories []string) (releases map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/stemcell_service.go --fake-name StemcellService . stemcellService
type stemcellService interface {
	FromTarball(path string) (stemcell interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/forms_service.go --fake-name FormsService . formsService
type formsService interface {
	FromDirectories(directories []string) (forms map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/instance_groups_service.go --fake-name InstanceGroupsService . instanceGroupsService
type instanceGroupsService interface {
	FromDirectories(directories []string) (instanceGroups map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/jobs_service.go --fake-name JobsService . jobsService
type jobsService interface {
	FromDirectories(directories []string) (jobs map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/properties_service.go --fake-name PropertiesService . propertiesService
type propertiesService interface {
	FromDirectories(directories []string) (properties map[string]interface{}, err error)
}

type Bake struct {
	metadataBuilder               metadataBuilder
	interpolator                  interpolator
	tileWriter                    tileWriter
	logger                        logger
	runtimeConfigsDirectoryReader directoryReader
	yamlMarshal                   func(interface{}) ([]byte, error)
	templateVariables             templateVariablesService
	releases                      releasesService
	stemcell                      stemcellService
	forms                         formsService
	instanceGroups                instanceGroupsService
	jobs                          jobsService
	properties                    propertiesService

	Options struct {
		Metadata           string   `short:"m"  long:"metadata"           required:"true" description:"path to the metadata file"`
		OutputFile         string   `short:"o"  long:"output-file"        required:"true" description:"path to where the tile will be output"`
		ReleaseDirectories []string `short:"rd" long:"releases-directory" required:"true" description:"path to a directory containing release tarballs"`

		BOSHVariableDirectories  []string `short:"vd"  long:"bosh-variables-directory"  description:"path to a directory containing BOSH variables"`
		EmbedPaths               []string `short:"e"   long:"embed"                     description:"path to files to include in the tile /embed directory"`
		FormDirectories          []string `short:"f"   long:"forms-directory"           description:"path to a directory containing forms"`
		IconPath                 string   `short:"i"   long:"icon"                      description:"path to icon file"`
		InstanceGroupDirectories []string `short:"ig"  long:"instance-groups-directory" description:"path to a directory containing instance groups"`
		JobDirectories           []string `short:"j"   long:"jobs-directory"            description:"path to a directory containing jobs"`
		MigrationDirectories     []string `short:"md"  long:"migrations-directory"      description:"path to a directory containing migrations"`
		PropertyDirectories      []string `short:"pd"  long:"properties-directory"      description:"path to a directory containing property blueprints"`
		RuntimeConfigDirectories []string `short:"rcd" long:"runtime-configs-directory" description:"path to a directory containing runtime configs"`
		StemcellTarball          string   `short:"st"  long:"stemcell-tarball"          description:"path to a stemcell tarball"`
		StubReleases             bool     `short:"sr"  long:"stub-releases"             description:"skips importing release tarballs into the tile"`
		VariableFiles            []string `short:"vf"  long:"variables-file"            description:"path to a file containing variables to interpolate"`
		Variables                []string `short:"vr"  long:"variable"                  description:"key value pairs of variables to interpolate"`
		Version                  string   `short:"v"   long:"version"                   description:"version of the tile"`
	}
}

func NewBake(
	metadataBuilder metadataBuilder,
	interpolator interpolator,
	tileWriter tileWriter,
	logger logger,
	runtimeConfigsDirectoryReader directoryReader,
	yamlMarshal func(interface{}) ([]byte, error),
	templateVariablesService templateVariablesService,
	releasesService releasesService,
	stemcellService stemcellService,
	formsService formsService,
	instanceGroupsService instanceGroupsService,
	jobsService jobsService,
	propertiesService propertiesService,
) Bake {

	return Bake{
		metadataBuilder: metadataBuilder,
		interpolator:    interpolator,
		tileWriter:      tileWriter,
		logger:          logger,
		runtimeConfigsDirectoryReader: runtimeConfigsDirectoryReader,
		yamlMarshal:                   yamlMarshal,
		templateVariables:             templateVariablesService,
		releases:                      releasesService,
		stemcell:                      stemcellService,
		forms:                         formsService,
		instanceGroups:                instanceGroupsService,
		jobs:                          jobsService,
		properties:                    propertiesService,
	}
}

func (b Bake) Execute(args []string) error {
	// NOTE: flag parsing and validation
	args, err := jhanda.Parse(&b.Options, args)
	if err != nil {
		return err
	}

	if len(b.Options.InstanceGroupDirectories) == 0 && len(b.Options.JobDirectories) > 0 {
		return errors.New("--jobs-directory flag requires --instance-groups-directory to also be specified")
	}

	b.logger.Printf("Creating metadata for %s...", b.Options.OutputFile)

	// NOTE: parsing variables files
	variables, err := b.templateVariables.FromPathsAndPairs(b.Options.VariableFiles, b.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}

	// NOTE: parsing releases
	b.logger.Println("Reading release manifests...")
	releaseManifests, err := b.releases.FromDirectories(b.Options.ReleaseDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse releases: %s", err)
	}

	// NOTE: reading stemcell manifest
	stemcellManifest, err := b.stemcell.FromTarball(b.Options.StemcellTarball)
	if err != nil {
		return fmt.Errorf("failed to parse stemcell: %s", err)
	}

	// NOTE: reading form files
	forms, err := b.forms.FromDirectories(b.Options.FormDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse forms: %s", err)
	}

	// NOTE: reading instance group files
	instanceGroups, err := b.instanceGroups.FromDirectories(b.Options.InstanceGroupDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse instance groups: %s", err)
	}

	// NOTE: reading job files
	jobs, err := b.jobs.FromDirectories(b.Options.JobDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse jobs: %s", err)
	}

	// NOTE: reading property files
	propertyBlueprints, err := b.properties.FromDirectories(b.Options.PropertyDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse properties: %s", err)
	}

	// NOTE: reading runtime config files
	var runtimeConfigs map[string]interface{}
	if b.Options.RuntimeConfigDirectories != nil {
		b.logger.Println("Reading runtime config files...")
		runtimeConfigs = map[string]interface{}{}
		for _, runtimeConfigsDir := range b.Options.RuntimeConfigDirectories {
			runtimeConfigsInDirectory, err := b.runtimeConfigsDirectoryReader.Read(runtimeConfigsDir)
			if err != nil {
				return err
			}

			for _, runtimeConfig := range runtimeConfigsInDirectory {
				runtimeConfigs[runtimeConfig.Name] = runtimeConfig.Metadata
			}
		}
	}

	// NOTE: parsing icon path
	var iconPath string
	if b.Options.IconPath != "" {
		b.logger.Println("Reading icon...")
		iconPath = b.Options.IconPath
	}

	// NOTE: generating metadata object representation
	generatedMetadata, err := b.metadataBuilder.Build(builder.BuildInput{
		IconPath:                iconPath,
		MetadataPath:            b.Options.Metadata,
		BOSHVariableDirectories: b.Options.BOSHVariableDirectories,
	})
	if err != nil {
		return err
	}

	// NOTE: marshalling metadata object to YAML
	b.logger.Println("Marshaling metadata file...")
	generatedMetadataYAML, err := b.yamlMarshal(generatedMetadata)
	if err != nil {
		return err
	}

	// NOTE: performing template interpolation on metadata YAML
	interpolatedMetadata, err := b.interpolator.Interpolate(builder.InterpolateInput{
		Version:            b.Options.Version,
		Variables:          variables,
		ReleaseManifests:   releaseManifests,
		StemcellManifest:   stemcellManifest,
		FormTypes:          forms,
		IconImage:          generatedMetadata.IconImage,
		InstanceGroups:     instanceGroups,
		Jobs:               jobs,
		PropertyBlueprints: propertyBlueprints,
		RuntimeConfigs:     runtimeConfigs,
	}, generatedMetadataYAML)
	if err != nil {
		return err
	}

	// NOTE: creating the output tile as a zip
	err = b.tileWriter.Write(interpolatedMetadata, builder.WriteInput{
		OutputFile:           b.Options.OutputFile,
		StubReleases:         b.Options.StubReleases,
		MigrationDirectories: b.Options.MigrationDirectories,
		ReleaseDirectories:   b.Options.ReleaseDirectories,
		EmbedPaths:           b.Options.EmbedPaths,
	})
	if err != nil {
		return err
	}

	return nil
}

func (b Bake) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
		ShortDescription: "bakes a tile",
		Flags:            b.Options,
	}
}
