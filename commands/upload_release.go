package commands

import (
	"fmt"
	"log"

	"github.com/Masterminds/semver"

	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"

	"github.com/pivotal-cf/kiln/builder"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4"
)

type UploadRelease struct {
	FS                    billy.Filesystem
	KilnfileLoader        KilnfileLoader
	ReleaseUploaderFinder ReleaseUploaderFinder
	Logger                *log.Logger

	Options struct {
		UploadTargetID string `           long:"upload-target-id" required:"true" description:"the ID of the release source where the built release will be uploaded"`
		LocalPath      string `short:"lp" long:"local-path"       required:"true" description:"path to BOSH release tarball"`

		Kilnfile       string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`
	}
}

//go:generate counterfeiter -o ./fakes/release_uploader_finder.go --fake-name ReleaseUploaderFinder . ReleaseUploaderFinder
type ReleaseUploaderFinder func(cargo.Kilnfile, string) (fetcher.ReleaseUploader, error)

func (command UploadRelease) Execute(args []string) error {
	_, err := jhanda.Parse(&command.Options, args)
	if err != nil {
		return err
	}

	kilnfile, _, err := command.KilnfileLoader.LoadKilnfiles(
		command.FS,
		command.Options.Kilnfile,
		command.Options.VariablesFiles,
		command.Options.Variables,
	)
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

	requirement := release.Requirement{Name: manifest.Name, Version: manifest.Version}
	_, found, err := releaseUploader.GetMatchedRelease(requirement)
	if err != nil {
		return fmt.Errorf("couldn't query release source: %w", err)
	}

	if found {
		return fmt.Errorf("a release with name %q and version %q already exists on %s",
			manifest.Name, manifest.Version, command.Options.UploadTargetID)
	}

	_, err = releaseUploader.UploadRelease(release.Requirement{
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
