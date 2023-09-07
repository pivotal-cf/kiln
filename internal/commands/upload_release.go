package commands

import (
	"fmt"
	"log"
	"os"

	"github.com/Masterminds/semver/v3"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type UploadRelease struct {
	FS                    billy.Filesystem
	ReleaseUploaderFinder ReleaseUploaderFinder
	Logger                *log.Logger

	Options struct {
		flags.Standard

		UploadTargetID string `           long:"upload-target-id" required:"true" description:"the ID of the release source where the built release will be uploaded"`
		LocalPath      string `short:"lp" long:"local-path"       required:"true" description:"path to BOSH release tarball"`
	}
}

//counterfeiter:generate -o ./fakes/release_uploader_finder.go --fake-name ReleaseUploaderFinder . ReleaseUploaderFinder
type ReleaseUploaderFinder func(cargo.Kilnfile, string) (component.ReleaseUploader, error)

func (command UploadRelease) Execute(args []string) error {
	_, err := flags.LoadWithDefaults(&command.Options, args, os.Stat)
	if err != nil {
		return err
	}

	kilnfile, _, err := command.Options.Standard.LoadKilnfiles(command.FS, nil)
	if err != nil {
		return fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	releaseUploader, err := command.ReleaseUploaderFinder(kilnfile, command.Options.UploadTargetID)
	if err != nil {
		return fmt.Errorf("error finding release source: %w", err)
	}

	releaseTarball, err := cargo.OpenBOSHReleaseTarball(command.Options.LocalPath)
	if err != nil {
		return fmt.Errorf("error reading the release manifest: %w", err)
	}

	version, err := semver.NewVersion(releaseTarball.Manifest.Version)
	if err != nil {
		return fmt.Errorf("error parsing release version %q: release version is not valid semver: %w", releaseTarball.Manifest.Version, err)
	}
	if version.Prerelease() != "" {
		return fmt.Errorf("cannot upload development release %q - only finalized releases are allowed", releaseTarball.Manifest.Version)
	}

	requirement := cargo.BOSHReleaseTarballSpecification{Name: releaseTarball.Manifest.Name, Version: releaseTarball.Manifest.Version}
	_, err = releaseUploader.GetMatchedRelease(requirement)
	if err != nil {
		if !component.IsErrNotFound(err) {
			return fmt.Errorf("couldn't query release source: %w", err)
		}
	} else {
		return fmt.Errorf("a release with name %q and version %q already exists on %s",
			releaseTarball.Manifest.Name, releaseTarball.Manifest.Version, command.Options.UploadTargetID)
	}

	file, err := os.Open(releaseTarball.FilePath)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(file)
	_, err = releaseUploader.UploadRelease(cargo.BOSHReleaseTarballSpecification{
		Name:    releaseTarball.Manifest.Name,
		Version: releaseTarball.Manifest.Version,
	}, file)
	if err != nil {
		return fmt.Errorf("error uploading the release: %w", err)
	}

	command.Logger.Println("Upload succeeded")

	return nil
}

func (command UploadRelease) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Uploads a BOSH Release to an S3 release source for use in kiln fetch",
		ShortDescription: "uploads a BOSH release to an s3 release_source",
		Flags:            command.Options,
	}
}
