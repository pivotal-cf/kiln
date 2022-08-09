package scenario

import (
	"context"
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

func iSetTheStemcellVersionInTheLockToMatchTheOneUsedForTheTile(ctx context.Context) (context.Context, error) {
	lockPath, err := kilnfileLockPath(ctx)
	if err != nil {
		return ctx, err
	}

	var lock cargo.KilnfileLock
	err = loadFileAsYAML(lockPath, &lock)
	if err != nil {
		return ctx, err
	}

	lock.Stemcell.Version, err = fetchAssociatedStemcellVersion(ctx, "hello")
	if err != nil {
		return ctx, err
	}

	err = saveAsYAML(lockPath, lock)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}
