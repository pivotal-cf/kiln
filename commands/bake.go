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

//go:generate counterfeiter -o ./fakes/part_reader.go --fake-name PartReader . partReader
type partReader interface {
	Read(path string) (builder.Part, error)
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

//go:generate counterfeiter -o ./fakes/template_variables_parser.go --fake-name TemplateVariablesParser . templateVariablesParser
type templateVariablesParser interface {
	Execute(paths []string, pairs []string) (variables map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/release_parser.go --fake-name ReleaseParser . releaseParser
type releaseParser interface {
	Execute(directories []string) (releases map[string]interface{}, err error)
}

type Bake struct {
	metadataBuilder                  metadataBuilder
	interpolator                     interpolator
	tileWriter                       tileWriter
	logger                           logger
	stemcellManifestReader           partReader
	formDirectoryReader              directoryReader
	instanceGroupDirectoryReader     directoryReader
	jobDirectoryReader               directoryReader
	propertyBlueprintDirectoryReader directoryReader
	runtimeConfigsDirectoryReader    directoryReader
	yamlMarshal                      func(interface{}) ([]byte, error)
	templateVariablesParser          templateVariablesParser
	releaseParser                    releaseParser

	Options struct {
		Metadata           string   `short:"m"  long:"metadata"           required:"true" description:"path to the metadata file"`
		OutputFile         string   `short:"o"  long:"output-file"        required:"true" description:"path to where the tile will be output"`
		IconPath           string   `short:"i"  long:"icon"               required:"true" description:"path to icon file"`
		ReleaseDirectories []string `short:"rd" long:"releases-directory" required:"true" description:"path to a directory containing release tarballs"`

		BOSHVariableDirectories  []string `short:"vd"  long:"bosh-variables-directory"  description:"path to a directory containing BOSH variables"`
		EmbedPaths               []string `short:"e"   long:"embed"                     description:"path to files to include in the tile /embed directory"`
		FormDirectories          []string `short:"f"   long:"forms-directory"           description:"path to a directory containing forms"`
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
	stemcellManifestReader partReader,
	formDirectoryReader directoryReader,
	instanceGroupDirectoryReader directoryReader,
	jobDirectoryReader directoryReader,
	propertyBlueprintDirectoryReader directoryReader,
	runtimeConfigsDirectoryReader directoryReader,
	yamlMarshal func(interface{}) ([]byte, error),
	templateVariablesParser templateVariablesParser,
	releaseParser releaseParser,
) Bake {

	return Bake{
		metadataBuilder:                  metadataBuilder,
		interpolator:                     interpolator,
		tileWriter:                       tileWriter,
		logger:                           logger,
		stemcellManifestReader:           stemcellManifestReader,
		formDirectoryReader:              formDirectoryReader,
		instanceGroupDirectoryReader:     instanceGroupDirectoryReader,
		jobDirectoryReader:               jobDirectoryReader,
		propertyBlueprintDirectoryReader: propertyBlueprintDirectoryReader,
		runtimeConfigsDirectoryReader:    runtimeConfigsDirectoryReader,
		yamlMarshal:                      yamlMarshal,
		templateVariablesParser:          templateVariablesParser,
		releaseParser:                    releaseParser,
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
	variables, err := b.templateVariablesParser.Execute(b.Options.VariableFiles, b.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}

	// NOTE: parsing releases
	b.logger.Println("Reading release manifests...")
	releaseManifests, err := b.releaseParser.Execute(b.Options.ReleaseDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse releases: %s", err)
	}

	// NOTE: reading stemcell manifest
	var stemcellManifest interface{}
	if b.Options.StemcellTarball != "" {
		b.logger.Println("Reading stemcell manifests...")
		stemcell, err := b.stemcellManifestReader.Read(b.Options.StemcellTarball)
		if err != nil {
			return err
		}
		stemcellManifest = stemcell.Metadata
	}

	// NOTE: reading form files
	var formTypes map[string]interface{}
	if b.Options.FormDirectories != nil {
		b.logger.Println("Reading form files...")
		formTypes = map[string]interface{}{}
		for _, formDir := range b.Options.FormDirectories {
			forms, err := b.formDirectoryReader.Read(formDir)
			if err != nil {
				return err
			}

			for _, form := range forms {
				formTypes[form.Name] = form.Metadata
			}
		}
	}

	// NOTE: reading instance group files
	var instanceGroups map[string]interface{}
	if b.Options.InstanceGroupDirectories != nil {
		b.logger.Println("Reading instance group files...")
		instanceGroups = map[string]interface{}{}
		for _, instanceGroupDir := range b.Options.InstanceGroupDirectories {
			instanceGroupsInDirectory, err := b.instanceGroupDirectoryReader.Read(instanceGroupDir)
			if err != nil {
				return err
			}

			for _, instanceGroup := range instanceGroupsInDirectory {
				instanceGroups[instanceGroup.Name] = instanceGroup.Metadata
			}
		}
	}

	// NOTE: reading job files
	var jobs map[string]interface{}
	if b.Options.JobDirectories != nil {
		b.logger.Println("Reading jobs files...")
		jobs = map[string]interface{}{}
		for _, jobsDir := range b.Options.JobDirectories {
			jobsInDirectory, err := b.jobDirectoryReader.Read(jobsDir)
			if err != nil {
				return err
			}

			for _, job := range jobsInDirectory {
				jobs[job.Name] = job.Metadata
			}
		}
	}

	// NOTE: reading property files
	var propertyBlueprints map[string]interface{}
	if b.Options.PropertyDirectories != nil {
		b.logger.Println("Reading property blueprint files...")
		propertyBlueprints = map[string]interface{}{}
		for _, propertyBlueprintDir := range b.Options.PropertyDirectories {
			propertyBlueprintsInDirectory, err := b.propertyBlueprintDirectoryReader.Read(propertyBlueprintDir)
			if err != nil {
				return err
			}

			for _, propertyBlueprint := range propertyBlueprintsInDirectory {
				propertyBlueprints[propertyBlueprint.Name] = propertyBlueprint.Metadata
			}
		}
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

	// NOTE: generating metadata object representation
	generatedMetadata, err := b.metadataBuilder.Build(builder.BuildInput{
		IconPath:                b.Options.IconPath,
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
		FormTypes:          formTypes,
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

func parseReleases(reader partReader, directories []string) (map[string]interface{}, error) {
	return NewReleaseParser(reader).Execute(directories)
}
