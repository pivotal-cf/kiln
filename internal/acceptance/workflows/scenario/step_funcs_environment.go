package scenario

import (
	"context"
	"os/exec"
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
		"--config", "hello-product-config.yml",
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
