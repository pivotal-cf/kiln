package commands

import (
	"fmt"
	"log"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/builder"
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
	_, err := flags.LoadFlagsWithDefaults(&command.Options, args, command.FS.Stat)
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

	file, err := command.FS.Open(command.Options.LocalPath)
	if err != nil {
		return fmt.Errorf("could not open release: %w", err)
	}

	manifestReader := builder.NewReleaseManifestReader(command.FS)
	part, err := manifestReader.Read(command.Options.LocalPath)
	if err != nil {
		return fmt.Errorf("error reading the release manifest: %w", err)
	}

	manifest := part.Metadata.(builder.ReleaseManifest)
	if manifest.StemcellOS != "" {
		return fmt.Errorf("cannot upload compiled release %q - only uncompiled releases are allowed", command.Options.LocalPath)
	}

	version, err := semver.NewVersion(manifest.Version)
	if err != nil {
		return fmt.Errorf("error parsing release version %q - release version is not valid semver", manifest.Version)
	}
	if version.Prerelease() != "" {
		return fmt.Errorf("cannot upload development release %q - only finalized releases are allowed", manifest.Version)
	}

	requirement := component.Requirement{Name: manifest.Name, Version: manifest.Version}
	_, found, err := releaseUploader.GetMatchedRelease(requirement)
	if err != nil {
		return fmt.Errorf("couldn't query release source: %w", err)
	}

	if found {
		return fmt.Errorf("a release with name %q and version %q already exists on %s",
			manifest.Name, manifest.Version, command.Options.UploadTargetID)
	}

	_, err = releaseUploader.UploadRelease(component.Requirement{
		Name:    manifest.Name,
		Version: manifest.Version,
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
