package scenario

import (
	"context"
	"os/exec"
)

func iUploadConfigureAndApplyTheTile(ctx context.Context) error {
	env, err := environment(ctx)
	if err != nil {
		return err
	}
	tilePath, err := defaultFilePathForTile(ctx)
	if err != nil {
		return err
	}
	version, err := tileVersion(ctx)
	if err != nil {
		return err
	}

	err = runAndLogOnError(ctx, exec.Command("om", "--skip-ssl-validation", "upload-product", "--product", tilePath))
	if err != nil {
		return err
	}
	err = runAndLogOnError(ctx, exec.Command("om", "--skip-ssl-validation", "stage-product", "--product-name", "hello", "--product-version", version))
	if err != nil {
		return err
	}
	err = runAndLogOnError(ctx, exec.Command("om", "--skip-ssl-validation", "configure-product",
		"--config", "hello-product-config.yml",
		"--var", "subnet="+env.ServiceSubnetName,
		"--var", "az="+env.AvailabilityZones[0],
	))
	if err != nil {
		return err
	}
	err = runAndLogOnError(ctx, exec.Command("om", "--skip-ssl-validation", "apply-changes"))
	if err != nil {
		return err
	}

	return nil
}
