package steps

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func loadS3Credentials() (keyID, accessKey string, err error) {
	keyID, err = loadEnvVar("AWS_ACCESS_KEY_ID", "required for s3 release source to cache releases")
	if err != nil {
		return
	}
	accessKey, err = loadEnvVar("AWS_SECRET_ACCESS_KEY", "required for s3 release source to cache releases")
	if err != nil {
		return
	}
	return
}

func iAddACompiledSReleaseSourceToTheKilnfile(ctx context.Context, bucketName string) error {
	keyID, accessKey, err := loadS3Credentials()
	kfPath, err := kilnfilePath(ctx)
	if err != nil {
		return err
	}

	var kf cargo.Kilnfile
	err = loadFileAsYAML(kfPath, &kf)
	if err != nil {
		return err
	}

	for _, rs := range kf.ReleaseSources {
		if rs.Bucket == bucketName {
			return nil
		}
	}

	kf.ReleaseSources = append(kf.ReleaseSources, cargo.ReleaseSourceConfig{
		Type:            component.ReleaseSourceTypeS3,
		Bucket:          bucketName,
		PathTemplate:    "{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz",
		Region:          "us-west-1",
		Publishable:     true,
		AccessKeyId:     keyID,
		SecretAccessKey: accessKey,
	})

	setPublishableReleaseSource(ctx, bucketName)

	return saveAsYAML(kfPath, kf)
}

func iInvokeKilnCacheCompiledReleases(ctx context.Context) error {
	token, err := githubToken(ctx)
	if err != nil {
		return err
	}
	uploadTargetID, err := publishableReleaseSource(ctx)
	if err != nil {
		return err
	}
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	env, err := environment(ctx)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "go", "run", "github.com/pivotal-cf/kiln", "cache-compiled-releases",
		"--variable", "github_token="+token,
		"--upload-target-id", uploadTargetID,
		"--name", "hello",
		"--om-username", env.OpsManager.Username,
		"--om-password", env.OpsManager.Password,
		"--om-target", env.OpsManager.URL,
		"--om-private-key", env.OpsManagerPrivateKey,
	)
	cmd.Dir = repoPath
	return runAndLogOnError(cmd)
}

func theStemcellVersionInTheLockMatchesTheUsedForTheTile(ctx context.Context) error {
	lockPath, err := kilnfileLockPath(ctx)
	if err != nil {
		return err
	}

	var stemcellAssociations struct {
		StemcellLibrary []struct {
			Version string `yaml:"version"`
		} `yaml:"stemcell_library"`
	}
	err = runAndParseStdoutAsYAML(
		exec.Command("om", "--skip-ssl-validation",
			"curl", "--path", "/api/v0/stemcell_associations",
		),
		&stemcellAssociations,
	)
	if len(stemcellAssociations.StemcellLibrary) == 0 {
		return fmt.Errorf("no stemcells found on ops manager")
	}
	var kl cargo.KilnfileLock
	err = loadFileAsYAML(lockPath, &kl)
	if err != nil {
		return err
	}
	kl.Stemcell.Version = stemcellAssociations.StemcellLibrary[0].Version
	return saveAsYAML(lockPath, kl)
}
