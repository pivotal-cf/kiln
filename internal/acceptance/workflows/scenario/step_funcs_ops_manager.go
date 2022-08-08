package scenario

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func iUploadConfigureAndApplyTheTile(ctx context.Context) (context.Context, error) {
	env, err := environment(ctx)
	if err != nil {
		return ctx, err
	}
	tilePath, err := defaultFilePathForTile(ctx)
	if err != nil {
		return ctx, err
	}
	version, err := tileVersion(ctx)
	if err != nil {
		return ctx, err
	}

	ctx, err = runAndLogOnError(ctx, exec.Command("om", "--skip-ssl-validation", "upload-product", "--product", tilePath), true)
	if err != nil {
		return ctx, err
	}
	ctx, err = runAndLogOnError(ctx, exec.Command("om", "--skip-ssl-validation", "stage-product", "--product-name", "hello", "--product-version", version), true)
	if err != nil {
		return ctx, err
	}
	ctx, err = runAndLogOnError(ctx, exec.Command("om", "--skip-ssl-validation", "configure-product",
		"--config", "scenario/fixtures/hello-product-config.yml",
		"--var", "subnet="+env.ServiceSubnetName,
		"--var", "az="+env.AvailabilityZones[0],
	), true)
	if err != nil {
		return ctx, err
	}
	ctx, err = runAndLogOnError(ctx, exec.Command("om", "--skip-ssl-validation", "apply-changes"), true)
	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

func theStemcellVersionInTheLockMatchesTheUsedForTheTile(ctx context.Context) (context.Context, error) {
	lockPath, err := kilnfileLockPath(ctx)
	if err != nil {
		return ctx, err
	}

	var stemcellAssociations struct {
		StemcellLibrary []struct {
			Version string `yaml:"version"`
		} `yaml:"stemcell_library"`
	}
	ctx, err = runAndParseStdoutAsYAML(ctx,
		exec.Command("om", "--skip-ssl-validation",
			"curl", "--path", "/api/v0/stemcell_associations",
		),
		&stemcellAssociations,
	)
	if err != nil {
		return ctx, err
	}
	if len(stemcellAssociations.StemcellLibrary) == 0 {
		return ctx, fmt.Errorf("no stemcells found on ops manager")
	}
	var kl cargo.KilnfileLock
	err = loadFileAsYAML(lockPath, &kl)
	if err != nil {
		return ctx, err
	}
	kl.Stemcell.Version = stemcellAssociations.StemcellLibrary[0].Version
	err = saveAsYAML(lockPath, kl)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}
