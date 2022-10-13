package commands

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/helper"
)

//counterfeiter:generate -o ./fakes/interpolator.go --fake-name Interpolator . interpolator
type interpolator interface {
	Interpolate(input builder.InterpolateInput, templateName string, templateYAML []byte) ([]byte, error)
}

//counterfeiter:generate -o ./fakes/tile_writer.go --fake-name TileWriter . tileWriter
type tileWriter interface {
	Write(generatedMetadataContents []byte, input builder.WriteInput) error
}

//counterfeiter:generate -o ./fakes/from_directories.go --fake-name FromDirectories . fromDirectories
type fromDirectories interface {
	FromDirectories(directories []string) (map[string]interface{}, error)
}

//counterfeiter:generate -o ./fakes/parse_metadata_templates.go --fake-name MetadataTemplatesParser . metadataTemplatesParser
type metadataTemplatesParser interface {
	ParseMetadataTemplates(directories []string, variables map[string]interface{}) (map[string]interface{}, error)
}

//counterfeiter:generate -o ./fakes/stemcell_service.go --fake-name StemcellService . stemcellService
type stemcellService interface {
	fromDirectories
	FromKilnfile(path string) (stemcell map[string]interface{}, err error)
	FromTarball(path string) (stemcell interface{}, err error)
}

//counterfeiter:generate -o ./fakes/template_variables_service.go --fake-name TemplateVariablesService . templateVariablesService
type templateVariablesService interface {
	FromPathsAndPairs(paths []string, pairs []string) (templateVariables map[string]interface{}, err error)
}

//counterfeiter:generate -o ./fakes/icon_service.go --fake-name IconService . iconService
type iconService interface {
	Encode(path string) (encodedIcon string, err error)
}

//counterfeiter:generate -o ./fakes/metadata_service.go --fake-name MetadataService . metadataService
type metadataService interface {
	Read(path string) (metadata []byte, err error)
}

//counterfeiter:generate -o ./fakes/checksummer.go --fake-name Checksummer . checksummer
type checksummer interface {
	Sum(path string) error
}

func NewBake(fs billy.Filesystem, releasesService baking.ReleasesService, outLogger *log.Logger, errLogger *log.Logger) *Bake {
	filesystem := helper.NewFilesystem()
	zipper := builder.NewZipper()
	interpolator := builder.NewInterpolator()
	tileWriter := builder.NewTileWriter(filesystem, &zipper, errLogger)

	stemcellManifestReader := builder.NewStemcellManifestReader(filesystem)
	stemcellService := baking.NewStemcellService(errLogger, stemcellManifestReader)

	templateVariablesService := baking.NewTemplateVariablesService(fs)

	iconService := baking.NewIconService(errLogger)

	metadataService := baking.NewMetadataService()
	checksummer := baking.NewChecksummer(errLogger)

	return &Bake{
		interpolator: interpolator,
		tileWriter:   tileWriter,
		checksummer:  checksummer,
		outLogger:    outLogger,
		errLogger:    errLogger,

		templateVariables: templateVariablesService,
		releases:          releasesService,
		stemcell:          stemcellService,
		icon:              iconService,

		metadata: metadataService,

		boshVariables:  builder.MetadataPartsDirectoryReader{},
		forms:          builder.MetadataPartsDirectoryReader{},
		instanceGroups: builder.MetadataPartsDirectoryReader{},
		jobs:           builder.MetadataPartsDirectoryReader{},
		properties:     builder.MetadataPartsDirectoryReader{},
		runtimeConfigs: builder.MetadataPartsDirectoryReader{},
	}
}

type Bake struct {
	interpolator      interpolator
	checksummer       checksummer
	tileWriter        tileWriter
	outLogger         *log.Logger
	errLogger         *log.Logger
	templateVariables templateVariablesService
	stemcell          stemcellService
	releases          fromDirectories

	boshVariables,
	forms,
	instanceGroups,
	jobs,
	properties,
	runtimeConfigs metadataTemplatesParser

	icon     iconService
	metadata metadataService

	Options struct {
		flags.Standard

		Metadata                 string   `short:"m"   long:"metadata"                  default-path:"base.yml"        description:"path to the metadata file"`
		ReleaseDirectories       []string `short:"rd"  long:"releases-directory"        default-path:"releases"        description:"path to a directory containing release tarballs"`
		FormDirectories          []string `short:"f"   long:"forms-directory"           default-path:"forms"           description:"path to a directory containing forms"`
		IconPath                 string   `short:"i"   long:"icon"                      default-path:"icon.png"        description:"path to icon file"`
		InstanceGroupDirectories []string `short:"ig"  long:"instance-groups-directory" default-path:"instance_groups" description:"path to a directory containing instance groups"`
		JobDirectories           []string `short:"j"   long:"jobs-directory"            default-path:"jobs"            description:"path to a directory containing jobs"`
		MigrationDirectories     []string `short:"md"  long:"migrations-directory"      default-path:"migrations"      description:"path to a directory containing migrations"`
		PropertyDirectories      []string `short:"pd"  long:"properties-directory"      default-path:"properties"      description:"path to a directory containing property blueprints"`
		RuntimeConfigDirectories []string `short:"rcd" long:"runtime-configs-directory" default-path:"runtime_configs" description:"path to a directory containing runtime configs"`
		BOSHVariableDirectories  []string `short:"vd"  long:"bosh-variables-directory"  default-path:"bosh_variables"  description:"path to a directory containing BOSH variables"`
		StemcellTarball          string   `short:"st"  long:"stemcell-tarball"                                         description:"deprecated -- path to a stemcell tarball  (NOTE: mutually exclusive with --kilnfile)"`
		StemcellsDirectories     []string `short:"sd"  long:"stemcells-directory"                                      description:"path to a directory containing stemcells  (NOTE: mutually exclusive with --kilnfile or --stemcell-tarball)"`
		EmbedPaths               []string `short:"e"   long:"embed"                                                    description:"path to files to include in the tile /embed directory"`
		OutputFile               string   `short:"o"   long:"output-file"                                              description:"path to where the tile will be output"`
		MetadataOnly             bool     `short:"mo"  long:"metadata-only"                                            description:"don't build a tile, output the metadata to stdout"`
		Sha256                   bool     `            long:"sha256"                                                   description:"calculates a SHA256 checksum of the output file"`
		StubReleases             bool     `short:"sr"  long:"stub-releases"                                            description:"skips importing release tarballs into the tile"`
		Version                  string   `short:"v"   long:"version"                                                  description:"version of the tile"`
	}
}

func NewBakeWithInterfaces(
	interpolator interpolator,
	tileWriter tileWriter,
	outLogger *log.Logger,
	errLogger *log.Logger,
	templateVariablesService templateVariablesService,
	boshVariablesService metadataTemplatesParser,
	releasesService fromDirectories,
	stemcellService stemcellService,
	formsService metadataTemplatesParser,
	instanceGroupsService metadataTemplatesParser,
	jobsService metadataTemplatesParser,
	propertiesService metadataTemplatesParser,
	runtimeConfigsService metadataTemplatesParser,
	iconService iconService,
	metadataService metadataService,
	checksummer checksummer,
) Bake {
	return Bake{
		interpolator:      interpolator,
		tileWriter:        tileWriter,
		checksummer:       checksummer,
		outLogger:         outLogger,
		errLogger:         errLogger,
		templateVariables: templateVariablesService,
		releases:          releasesService,
		stemcell:          stemcellService,
		icon:              iconService,
		metadata:          metadataService,

		boshVariables:  boshVariablesService,
		forms:          formsService,
		instanceGroups: instanceGroupsService,
		jobs:           jobsService,
		properties:     propertiesService,
		runtimeConfigs: runtimeConfigsService,
	}
}

func shouldGenerateTileFileName(b *Bake, args []string) bool {
	return b.Options.OutputFile == "" &&
		!b.Options.MetadataOnly &&
		!flags.IsSet("o", "output-file", args)
}

func shouldReadVersionFile(b *Bake, args []string) bool {
	return b.Options.Version == "" && !flags.IsSet("v", "version", args)
}

func shouldNotUseDefaultKilnfileFlag(args []string) bool {
	return (flags.IsSet("st", "stemcell-tarball", args) || flags.IsSet("sd", "stemcells-directory", args)) &&
		!flags.IsSet("kf", "kilnfile", args)
}

func (b *Bake) loadFlags(args []string, stat flags.StatFunc, readFile func(string) ([]byte, error)) error {
	_, err := flags.LoadFlagsWithDefaults(&b.Options, args, stat)
	if err != nil {
		return err
	}

	if shouldReadVersionFile(b, args) {
		versionBuf, _ := readFile("version")
		b.Options.Version = strings.TrimSpace(string(versionBuf))
	}

	if shouldGenerateTileFileName(b, args) {
		b.Options.OutputFile = "tile.pivotal"
		if b.Options.Version != "" {
			b.Options.OutputFile = "tile-" + b.Options.Version + ".pivotal"
		}
	}

	if shouldNotUseDefaultKilnfileFlag(args) {
		b.Options.Standard.Kilnfile = ""
	}

	return nil
}

func (b *Bake) Execute(args []string) error {
	err := b.loadFlags(args, os.Stat, os.ReadFile)
	if err != nil {
		return err
	}

	if b.Options.Metadata == "" {
		return errors.New("missing required flag \"--metadata\"")
	}

	if len(b.Options.InstanceGroupDirectories) == 0 && len(b.Options.JobDirectories) > 0 {
		return errors.New("--jobs-directory flag requires --instance-groups-directory to also be specified")
	}

	if b.Options.Kilnfile != "" && b.Options.StemcellTarball != "" {
		return errors.New("--kilnfile cannot be provided when using --stemcell-tarball")
	}

	if b.Options.Kilnfile != "" && len(b.Options.StemcellsDirectories) > 0 {
		return errors.New("--kilnfile cannot be provided when using --stemcells-directory")
	}

	if b.Options.StemcellTarball != "" && len(b.Options.StemcellsDirectories) > 0 {
		return errors.New("--stemcell-tarball cannot be provided when using --stemcells-directory")
	}

	if b.Options.OutputFile != "" && b.Options.MetadataOnly {
		return errors.New("--output-file cannot be provided when using --metadata-only")
	}

	// TODO: Remove check after deprecation of --stemcell-tarball
	if b.Options.StemcellTarball != "" {
		b.errLogger.Println("warning: --stemcell-tarball is being deprecated in favor of --stemcells-directory")
	}

	templateVariables, err := b.templateVariables.FromPathsAndPairs(b.Options.VariableFiles, b.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
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
	} else if b.Options.Kilnfile != "" {
		stemcellManifests, err = b.stemcell.FromKilnfile(b.Options.Kilnfile)
	} else if len(b.Options.StemcellsDirectories) > 0 {
		stemcellManifests, err = b.stemcell.FromDirectories(b.Options.StemcellsDirectories)
	}
	if err != nil {
		return fmt.Errorf("failed to parse stemcell: %s", err)
	}

	boshVariables, err := b.boshVariables.ParseMetadataTemplates(b.Options.BOSHVariableDirectories, templateVariables)
	if err != nil {
		return fmt.Errorf("failed to parse bosh variables: %s", err)
	}

	forms, err := b.forms.ParseMetadataTemplates(b.Options.FormDirectories, templateVariables)
	if err != nil {
		return fmt.Errorf("failed to parse forms: %s", err)
	}

	instanceGroups, err := b.instanceGroups.ParseMetadataTemplates(b.Options.InstanceGroupDirectories, templateVariables)
	if err != nil {
		return fmt.Errorf("failed to parse instance groups: %s", err)
	}

	jobs, err := b.jobs.ParseMetadataTemplates(b.Options.JobDirectories, templateVariables)
	if err != nil {
		return fmt.Errorf("failed to parse jobs: %s", err)
	}

	propertyBlueprints, err := b.properties.ParseMetadataTemplates(b.Options.PropertyDirectories, templateVariables)
	if err != nil {
		return fmt.Errorf("failed to parse properties: %s", err)
	}

	runtimeConfigs, err := b.runtimeConfigs.ParseMetadataTemplates(b.Options.RuntimeConfigDirectories, templateVariables)
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
		StemcellManifest:   stemcellManifest, // TODO Remove when --stemcell-tarball is deprecated
		FormTypes:          forms,
		IconImage:          icon,
		InstanceGroups:     instanceGroups,
		Jobs:               jobs,
		PropertyBlueprints: propertyBlueprints,
		RuntimeConfigs:     runtimeConfigs,
		StubReleases:       b.Options.StubReleases,
		MetadataGitSHA:     builder.GitMetadataSHA(filepath.Dir(b.Options.Kilnfile), b.Options.MetadataOnly || b.Options.StubReleases),
	}, b.Options.Metadata, metadata)
	if err != nil {
		return err
	}

	if b.Options.MetadataOnly {
		b.outLogger.Printf("%s", interpolatedMetadata)
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

func (b *Bake) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.",
		ShortDescription: "bakes a tile",
		Flags:            b.Options,
	}
}
