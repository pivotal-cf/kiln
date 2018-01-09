package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/pivotal-cf/jhanda/commands"
	"github.com/pivotal-cf/jhanda/flags"
	"github.com/pivotal-cf/kiln/builder"
)

type BakeConfig struct {
	BOSHVariableDirectories  flags.StringSlice `short:"vd"   long:"bosh-variables-directory"   description:"path to a directory containing BOSH variables"`
	EmbedPaths               flags.StringSlice `short:"e"    long:"embed"                      description:"path to files to include in the tile /embed directory"`
	FormDirectories          flags.StringSlice `short:"f"    long:"forms-directory"            description:"path to a directory containing forms"`
	IconPath                 string            `short:"i"    long:"icon"                       description:"path to icon file"`
	InstanceGroupDirectories flags.StringSlice `short:"ig"   long:"instance-groups-directory"  description:"path to a directory containing instance groups"`
	JobDirectories           flags.StringSlice `short:"j"    long:"jobs-directory"             description:"path to a directory containing jobs"`
	Metadata                 string            `short:"m"    long:"metadata"                   description:"path to the metadata file"`
	MigrationDirectories     flags.StringSlice `short:"md"   long:"migrations-directory"       description:"path to a directory containing migrations"`
	OutputFile               string            `short:"o"    long:"output-file"                description:"path to where the tile will be output"`
	PropertyDirectories      flags.StringSlice `short:"pd"   long:"properties-directory"       description:"path to a directory containing property blueprints"`
	ReleaseDirectories       flags.StringSlice `short:"rd"   long:"releases-directory"         description:"path to a directory containing release tarballs"`
	RuntimeConfigDirectories flags.StringSlice `short:"rcd"  long:"runtime-configs-directory"  description:"path to a directory containing runtime configs"`
	StemcellTarball          string            `short:"st"   long:"stemcell-tarball"           description:"path to a stemcell tarball"`
	StubReleases             bool              `short:"sr"   long:"stub-releases"              description:"skips importing release tarballs into the tile"`
	VariableFiles            flags.StringSlice `short:"vf"   long:"variables-file"             description:"path to a file containing variables to interpolate"`
	Variables                flags.StringSlice `short:"vr"   long:"variable"                   description:"key value pairs of variables to interpolate"`
	Version                  string            `short:"v"    long:"version"                    description:"version of the tile"`
}

type Bake struct {
	metadataBuilder                  metadataBuilder
	interpolator                     interpolator
	tileWriter                       tileWriter
	logger                           logger
	releaseManifestReader            partReader
	stemcellManifestReader           partReader
	formDirectoryReader              directoryReader
	instanceGroupDirectoryReader     directoryReader
	jobDirectoryReader               directoryReader
	propertyBlueprintDirectoryReader directoryReader
	runtimeConfigsDirectoryReader    directoryReader
	Options                          BakeConfig
}

var yamlMarshal = yaml.Marshal

//go:generate counterfeiter -o ./fakes/interpolator.go --fake-name Interpolator . interpolator

type interpolator interface {
	Interpolate(input builder.InterpolateInput, templateYAML []byte) ([]byte, error)
}

//go:generate counterfeiter -o ./fakes/tile_writer.go --fake-name TileWriter . tileWriter

type tileWriter interface {
	Write(productName string, generatedMetadataContents []byte, input builder.WriteInput) error
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

func NewBake(metadataBuilder metadataBuilder,
	interpolator interpolator,
	tileWriter tileWriter,
	logger logger,
	releaseManifestReader partReader,
	stemcellManifestReader partReader,
	formDirectoryReader directoryReader,
	instanceGroupDirectoryReader directoryReader,
	jobDirectoryReader directoryReader,
	propertyBlueprintDirectoryReader directoryReader,
	runtimeConfigsDirectoryReader directoryReader,
) Bake {
	return Bake{
		metadataBuilder:                  metadataBuilder,
		interpolator:                     interpolator,
		tileWriter:                       tileWriter,
		logger:                           logger,
		releaseManifestReader:            releaseManifestReader,
		stemcellManifestReader:           stemcellManifestReader,
		formDirectoryReader:              formDirectoryReader,
		instanceGroupDirectoryReader:     instanceGroupDirectoryReader,
		jobDirectoryReader:               jobDirectoryReader,
		propertyBlueprintDirectoryReader: propertyBlueprintDirectoryReader,
		runtimeConfigsDirectoryReader:    runtimeConfigsDirectoryReader,
	}
}

func (b Bake) Execute(args []string) error {
	config, err := b.parseArgs(args)
	if err != nil {
		return err
	}

	b.logger.Printf("Creating metadata for %s...", config.OutputFile)

	variables := map[string]string{}
	for _, file := range config.VariableFiles {
		err := b.readVariableFiles(file, variables)
		if err != nil {
			return fmt.Errorf("failed reading variable file: %s", err.Error())
		}
	}

	releaseTarballs, err := b.extractReleaseTarballFilenames(config)
	if err != nil {
		return err
	}

	b.logger.Println("Reading release manifests...")
	releaseManifests := map[string]interface{}{}
	for _, releaseTarball := range releaseTarballs {
		releaseManifest, err := b.releaseManifestReader.Read(releaseTarball)
		if err != nil {
			return err
		}
		releaseManifests[releaseManifest.Name] = releaseManifest.Metadata
	}

	var stemcellManifest interface{}
	if config.StemcellTarball != "" {
		b.logger.Println("Reading stemcell manifests...")
		stemcell, err := b.stemcellManifestReader.Read(config.StemcellTarball)
		if err != nil {
			return err
		}
		stemcellManifest = stemcell.Metadata
	}

	var formTypes map[string]interface{}
	if config.FormDirectories != nil {
		b.logger.Println("Reading form files...")
		formTypes = map[string]interface{}{}
		for _, formDir := range config.FormDirectories {
			forms, err := b.formDirectoryReader.Read(formDir)
			if err != nil {
				return err
			}

			for _, form := range forms {
				formTypes[form.Name] = form.Metadata
			}
		}
	}

	var instanceGroups map[string]interface{}
	if config.InstanceGroupDirectories != nil {
		b.logger.Println("Reading instance group files...")
		instanceGroups = map[string]interface{}{}
		for _, instanceGroupDir := range config.InstanceGroupDirectories {
			instanceGroupsInDirectory, err := b.instanceGroupDirectoryReader.Read(instanceGroupDir)
			if err != nil {
				return err
			}

			for _, instanceGroup := range instanceGroupsInDirectory {
				instanceGroups[instanceGroup.Name] = instanceGroup.Metadata
			}
		}
	}

	var jobs map[string]interface{}
	if config.JobDirectories != nil {
		b.logger.Println("Reading jobs files...")
		jobs = map[string]interface{}{}
		for _, jobsDir := range config.JobDirectories {
			jobsInDirectory, err := b.jobDirectoryReader.Read(jobsDir)
			if err != nil {
				return err
			}

			for _, job := range jobsInDirectory {
				jobs[job.Name] = job.Metadata
			}
		}
	}

	var propertyBlueprints map[string]interface{}
	if config.PropertyDirectories != nil {
		b.logger.Println("Reading property blueprint files...")
		propertyBlueprints = map[string]interface{}{}
		for _, propertyBlueprintDir := range config.PropertyDirectories {
			propertyBlueprintsInDirectory, err := b.propertyBlueprintDirectoryReader.Read(propertyBlueprintDir)
			if err != nil {
				return err
			}

			for _, propertyBlueprint := range propertyBlueprintsInDirectory {
				propertyBlueprints[propertyBlueprint.Name] = propertyBlueprint.Metadata
			}
		}
	}

	var runtimeConfigs map[string]interface{}
	if config.RuntimeConfigDirectories != nil {
		b.logger.Println("Reading runtime config files...")
		runtimeConfigs = map[string]interface{}{}
		for _, runtimeConfigsDir := range config.RuntimeConfigDirectories {
			runtimeConfigsInDirectory, err := b.runtimeConfigsDirectoryReader.Read(runtimeConfigsDir)
			if err != nil {
				return err
			}

			for _, runtimeConfig := range runtimeConfigsInDirectory {
				runtimeConfigs[runtimeConfig.Name] = runtimeConfig.Metadata
			}
		}
	}

	err = b.addVariablesToMap(config.Variables, variables)
	if err != nil {
		return err
	}

	buildInput := builder.BuildInput{
		IconPath:                config.IconPath,
		MetadataPath:            config.Metadata,
		BOSHVariableDirectories: config.BOSHVariableDirectories,
	}

	generatedMetadata, err := b.metadataBuilder.Build(buildInput)
	if err != nil {
		return err
	}

	b.logger.Println("Marshaling metadata file...")

	generatedMetadataYAML, err := yamlMarshal(generatedMetadata)
	if err != nil {
		return err
	}

	interpolatedMetadata, err := b.interpolator.Interpolate(builder.InterpolateInput{
		Version:            config.Version,
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

	writeInput := builder.WriteInput{
		OutputFile:           config.OutputFile,
		StubReleases:         config.StubReleases,
		MigrationDirectories: config.MigrationDirectories,
		ReleaseDirectories:   config.ReleaseDirectories,
		EmbedPaths:           config.EmbedPaths,
	}

	err = b.tileWriter.Write(generatedMetadata.Name, interpolatedMetadata, writeInput)
	if err != nil {
		return err
	}

	return nil
}

func (b Bake) Usage() commands.Usage {
	return commands.Usage{
		Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
		ShortDescription: "bakes a tile",
		Flags:            b.Options,
	}
}

func (b Bake) parseArgs(args []string) (BakeConfig, error) {
	config := BakeConfig{}

	args, err := flags.Parse(&config, args)
	if err != nil {
		return config, err
	}

	if len(config.ReleaseDirectories) == 0 {
		return config, errors.New("Please specify release tarballs directory with the --releases-directory parameter")
	}

	if config.IconPath == "" {
		return config, errors.New("--icon is a required parameter")
	}

	if config.Metadata == "" {
		return config, errors.New("--metadata is a required parameter")
	}

	if config.OutputFile == "" {
		return config, errors.New("--output-file is a required parameter")
	}

	if len(config.InstanceGroupDirectories) == 0 && len(config.JobDirectories) > 0 {
		return config, errors.New("--jobs-directory flag requires --instance-groups-directory to also be specified")
	}

	return config, nil
}

func (b Bake) addVariablesToMap(flagVariables []string, variables map[string]string) error {
	for _, variable := range flagVariables {
		variablePair := strings.SplitN(variable, "=", 2)
		if len(variablePair) < 2 {
			return errors.New("variable needs a key value in the form of key=value")
		}
		variables[variablePair[0]] = variablePair[1]
	}

	return nil
}

func (b Bake) extractReleaseTarballFilenames(config BakeConfig) ([]string, error) {
	var releaseTarballs []string

	for _, releasesDirectory := range config.ReleaseDirectories {
		files, err := ioutil.ReadDir(releasesDirectory)
		if err != nil {
			return []string{}, err
		}

		for _, file := range files {
			matchTarballs, _ := regexp.MatchString("tgz$|tar.gz$", file.Name())
			if !matchTarballs {
				continue
			}

			releaseTarballs = append(releaseTarballs, filepath.Join(releasesDirectory, file.Name()))
		}
	}

	return releaseTarballs, nil
}

func (b Bake) readVariableFiles(path string, variables map[string]string) error {
	variableData, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(variableData, &variables)
	if err != nil {
		return err
	}
	return nil
}
