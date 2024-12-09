package planitest

import (
	"errors"
	"io"
	"os"

	"github.com/pivotal-cf/kiln/pkg/planitest/internal"
)

type ProductConfig struct {
	TileFile   io.ReadSeeker
	ConfigFile io.ReadSeeker
}

type ProductService struct {
	config        ProductConfig
	renderService internal.RenderService
}

func NewProductService(config ProductConfig) (*ProductService, error) {
	if config.TileFile == nil {
		return nil, errors.New("Tile file must be provided")
	}

	if config.ConfigFile == nil {
		return nil, errors.New("Config file must be provided")
	}

	executor := internal.NewEnvironmentSharingCommandRunner(os.Environ())
	omRunner := internal.NewOMRunner(executor, internal.RealIO)
	opsManifestRunner := internal.NewOpsManifestRunner(executor, internal.RealIO, opsManifestAdditionalArgs()...)

	var (
		renderService internal.RenderService
		err           error
	)

	switch os.Getenv("RENDERER") {
	case "om":
		renderService, err = internal.NewOMServiceWithRunner(omRunner)
	case "ops-manifest":
		renderService, err = internal.NewOpsManifestServiceWithRunner(opsManifestRunner, internal.RealIO)
	default:
		err = errors.New("RENDERER must be set to om or ops-manifest")
	}
	if err != nil {
		return nil, err
	}

	return &ProductService{config: config, renderService: renderService}, nil
}

func opsManifestAdditionalArgs() []string {
	tasMetadataPath := os.Getenv("TAS_METADATA_PATH")
	tasConfigFile := os.Getenv("TAS_CONFIG_FILE")
	dollarOverridesFile := os.Getenv("DOLLAR_OVERRIDES_FILE")

	var args []string

	if tasMetadataPath != "" {
		args = append(args, "--tas-metadata-path", tasMetadataPath)
	}

	if tasConfigFile != "" {
		args = append(args, "--tas-config-file", tasConfigFile)
	}

	if dollarOverridesFile != "" {
		args = append(args, "--dollar-accessor-values-file", dollarOverridesFile)
	}

	return args
}

func (p *ProductService) RenderManifest(additionalProperties map[string]any) (Manifest, error) {
	_, err := p.config.ConfigFile.Seek(0, 0)
	if err != nil {
		return "", err
	}

	_, err = p.config.TileFile.Seek(0, 0)
	if err != nil {
		return "", err
	}

	tileConfig, err := internal.MergeAdditionalProductProperties(p.config.ConfigFile, additionalProperties)
	if err != nil {
		return "", err
	}

	m, err := p.renderService.RenderManifest(tileConfig, p.config.TileFile)
	if err != nil {
		return "", err
	}

	return Manifest(m), nil
}
