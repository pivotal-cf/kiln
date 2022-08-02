package scenario

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

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
	err = runAndParseStdoutAsYAML(ctx,
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
