package commands

import (
	"errors"
	"fmt"
	"log"

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

//go:generate counterfeiter -o ./fakes/bosh_variables_service.go --fake-name BOSHVariablesService . boshVariablesService
type boshVariablesService interface {
	FromDirectories(directories []string) (boshVariables map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/releases_service.go --fake-name ReleasesService . releasesService
type releasesService interface {
	FromDirectories(directories []string) (releases map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/stemcell_service.go --fake-name StemcellService . stemcellService
type stemcellService interface {
	FromDirectories(directories []string) (stemcell map[string]interface{}, err error)
	FromAssetsFile(path string) (stemcell map[string]interface{}, err error)
	FromTarball(path string) (stemcell interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/template_variables_service.go --fake-name TemplateVariablesService . templateVariablesService
type templateVariablesService interface {
	FromPathsAndPairs(paths []string, pairs []string) (templateVariables map[string]interface{}, err error)
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

//go:generate counterfeiter -o ./fakes/runtime_configs_service.go --fake-name RuntimeConfigsService . runtimeConfigsService
type runtimeConfigsService interface {
	FromDirectories(directories []string) (runtimeConfigs map[string]interface{}, err error)
}

//go:generate counterfeiter -o ./fakes/icon_service.go --fake-name IconService . iconService
type iconService interface {
	Encode(path string) (encodedIcon string, err error)
}

//go:generate counterfeiter -o ./fakes/metadata_service.go --fake-name MetadataService . metadataService
type metadataService interface {
	Read(path string) (metadata []byte, err error)
}

//go:generate counterfeiter -o ./fakes/checksummer.go --fake-name Checksummer . checksummer
type checksummer interface {
	Sum(path string) error
}

type Bake struct {
	interpolator      interpolator
	checksummer       checksummer
	tileWriter        tileWriter
	output            *log.Logger
	templateVariables templateVariablesService
	boshVariables     boshVariablesService
	releases          releasesService
	stemcell          stemcellService
	forms             formsService
	instanceGroups    instanceGroupsService
	jobs              jobsService
	properties        propertiesService
	runtimeConfigs    runtimeConfigsService
	icon              iconService
	metadata          metadataService

	Options struct {
		AssetsFile         string   `short:"a"  long:"assets-file"                        description:"path to assets file  (NOTE: mutually exclusive with --stemcell-directory)"`
		Metadata           string   `short:"m"  long:"metadata"           required:"true" description:"path to the metadata file"`
		OutputFile         string   `short:"o"  long:"output-file"                        description:"path to where the tile will be output"`
		ReleaseDirectories []string `short:"rd" long:"releases-directory"               description:"path to a directory containing release tarballs"`

		BOSHVariableDirectories  []string `short:"vd"  long:"bosh-variables-directory"  description:"path to a directory containing BOSH variables"`
		EmbedPaths               []string `short:"e"   long:"embed"                     description:"path to files to include in the tile /embed directory"`
		FormDirectories          []string `short:"f"   long:"forms-directory"           description:"path to a directory containing forms"`
		IconPath                 string   `short:"i"   long:"icon"                      description:"path to icon file"`
		InstanceGroupDirectories []string `short:"ig"  long:"instance-groups-directory" description:"path to a directory containing instance groups"`
		JobDirectories           []string `short:"j"   long:"jobs-directory"            description:"path to a directory containing jobs"`
		MetadataOnly             bool     `short:"mo"  long:"metadata-only"             description:"don't build a tile, output the metadata to stdout"`
		MigrationDirectories     []string `short:"md"  long:"migrations-directory"      description:"path to a directory containing migrations"`
		PropertyDirectories      []string `short:"pd"  long:"properties-directory"      description:"path to a directory containing property blueprints"`
		RuntimeConfigDirectories []string `short:"rcd" long:"runtime-configs-directory" description:"path to a directory containing runtime configs"`
		Sha256                   bool     `            long:"sha256"                    description:"calculates a SHA256 checksum of the output file"`
		StemcellTarball          string   `short:"st"  long:"stemcell-tarball"          description:"deprecated -- path to a stemcell tarball  (NOTE: mutually exclusive with --assets-file)"`
		StemcellsDirectories     []string `short:"sd"  long:"stemcells-directory"       description:"path to a directory containing stemcells  (NOTE: mutually exclusive with --assets-file or --stemcell-tarball)"`
		StubReleases             bool     `short:"sr"  long:"stub-releases"             description:"skips importing release tarballs into the tile"`
		VariableFiles            []string `short:"vf"  long:"variables-file"            description:"path to a file containing variables to interpolate"`
		Variables                []string `short:"vr"  long:"variable"                  description:"key value pairs of variables to interpolate"`
		Version                  string   `short:"v"   long:"version"                   description:"version of the tile"`
	}
}

func NewBake(
	interpolator interpolator,
	tileWriter tileWriter,
	output *log.Logger,
	templateVariablesService templateVariablesService,
	boshVariablesService boshVariablesService,
	releasesService releasesService,
	stemcellService stemcellService,
	formsService formsService,
	instanceGroupsService instanceGroupsService,
	jobsService jobsService,
	propertiesService propertiesService,
	runtimeConfigsService runtimeConfigsService,
	iconService iconService,
	metadataService metadataService,
	checksummer checksummer,
) Bake {

	return Bake{
		interpolator:      interpolator,
		tileWriter:        tileWriter,
		checksummer:       checksummer,
		output:            output,
		templateVariables: templateVariablesService,
		boshVariables:     boshVariablesService,
		releases:          releasesService,
		stemcell:          stemcellService,
		forms:             formsService,
		instanceGroups:    instanceGroupsService,
		jobs:              jobsService,
		properties:        propertiesService,
		runtimeConfigs:    runtimeConfigsService,
		icon:              iconService,
		metadata:          metadataService,
	}
}

func (b Bake) Execute(args []string) error {
	args, err := jhanda.Parse(&b.Options, args)
	if err != nil {
		return err
	}

	if len(b.Options.InstanceGroupDirectories) == 0 && len(b.Options.JobDirectories) > 0 {
		return errors.New("--jobs-directory flag requires --instance-groups-directory to also be specified")
	}

	if b.Options.OutputFile == "" && !b.Options.MetadataOnly {
		return errors.New("--output-file must be provided unless using --metadata-only")
	}

	if b.Options.AssetsFile != "" && b.Options.StemcellTarball != "" {
		return errors.New("--assets-file cannot be provided when using --stemcell-tarball")
	}

	if b.Options.AssetsFile != "" && len(b.Options.StemcellsDirectories) > 0 {
		return errors.New("--assets-file cannot be provided when using --stemcells-directory")
	}

	if b.Options.StemcellTarball != "" && len(b.Options.StemcellsDirectories) > 0 {
		return errors.New("--stemcell-tarball cannot be provided when using --stemcells-directory")
	}

	if b.Options.OutputFile != "" && b.Options.MetadataOnly {
		return errors.New("--output-file cannot be provided when using --metadata-only")
	}

	// TODO: Remove check after deprecation of --stemcell-tarball
	if b.Options.StemcellTarball != "" {
		b.output.Println("warning: --stemcell-tarball is being deprecated in favor of --stemcells-directory")
	}

	releaseManifests, err := b.releases.FromDirectories(b.Options.ReleaseDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse releases: %s", err)
	}

	var stemcellManifests map[string]interface{}
	var stemcellManifest interface{}
	if b.Options.StemcellTarball != "" {
		// TODO remove when stemcell tarball is deprecated
		stemcellManifest, err = b.stemcell.FromTarball(b.Options.StemcellTarball)
	} else if b.Options.AssetsFile != "" {
		stemcellManifests, err = b.stemcell.FromAssetsFile(b.Options.AssetsFile)
	} else if len(b.Options.StemcellsDirectories) > 0 {
		stemcellManifests, err = b.stemcell.FromDirectories(b.Options.StemcellsDirectories)
	}
	if err != nil {
		return fmt.Errorf("failed to parse stemcell: %s", err)
	}

	templateVariables, err := b.templateVariables.FromPathsAndPairs(b.Options.VariableFiles, b.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}

	boshVariables, err := b.boshVariables.FromDirectories(b.Options.BOSHVariableDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse bosh variables: %s", err)
	}

	forms, err := b.forms.FromDirectories(b.Options.FormDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse forms: %s", err)
	}

	instanceGroups, err := b.instanceGroups.FromDirectories(b.Options.InstanceGroupDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse instance groups: %s", err)
	}

	jobs, err := b.jobs.FromDirectories(b.Options.JobDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse jobs: %s", err)
	}

	propertyBlueprints, err := b.properties.FromDirectories(b.Options.PropertyDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse properties: %s", err)
	}

	runtimeConfigs, err := b.runtimeConfigs.FromDirectories(b.Options.RuntimeConfigDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse runtime configs: %s", err)
	}

	icon, err := b.icon.Encode(b.Options.IconPath)
	if err != nil {
		return fmt.Errorf("failed to encode icon: %s", err)
	}

	metadata, err := b.metadata.Read(b.Options.Metadata)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %s", err)
	}

	interpolatedMetadata, err := b.interpolator.Interpolate(builder.InterpolateInput{
		Version:            b.Options.Version,
		Variables:          templateVariables,
		BOSHVariables:      boshVariables,
		ReleaseManifests:   releaseManifests,
		StemcellManifests:  stemcellManifests,
		StemcellManifest:   stemcellManifest, //TODO Remove when --stemcell-tarball is deprecated
		FormTypes:          forms,
		IconImage:          icon,
		InstanceGroups:     instanceGroups,
		Jobs:               jobs,
		PropertyBlueprints: propertyBlueprints,
		RuntimeConfigs:     runtimeConfigs,
		StubReleases:       b.Options.StubReleases,
	}, metadata)
	if err != nil {
		return err
	}

	if b.Options.MetadataOnly {
		b.output.Printf("%s", interpolatedMetadata)
		return nil
	}

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

	if b.Options.Sha256 {
		err = b.checksummer.Sum(b.Options.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum: %s", err)
		}
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
