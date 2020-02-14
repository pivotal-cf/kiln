package commands

import (
	"fmt"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/release"
	"log"

	"github.com/pivotal-cf/kiln/builder"

	"gopkg.in/src-d/go-billy.v4"
)

type UploadReleaseCmd struct {
	TargetSourceID string `short:"s" long:"release-source" required:"true" description:"name of the release source specified in the Kilnfile"`
	LocalPath      string `short:"l" long:"local-path"     required:"true" description:"path to BOSH release tarball"`
	panicCommand
}

//go:generate counterfeiter -o ./fakes/release_uploader_finder.go --fake-name ReleaseUploaderFinder . ReleaseUploaderFinder
type ReleaseUploaderFinder func(string) (fetcher.ReleaseUploader, error)

func (u UploadReleaseCmd) Runner(deps Dependencies) (CommandRunner, error) {
	return UploadRelease{
		TargetSourceID:        u.TargetSourceID,
		LocalPath:             u.LocalPath,
		FS:                    deps.Filesystem,
		ReleaseUploaderFinder: deps.ReleaseSourceRepo.FindReleaseUploader,
		Logger:                deps.OutLogger,
	}, nil
}

type UploadRelease struct {
	TargetSourceID string
	LocalPath      string

	FS                    billy.Filesystem
	ReleaseUploaderFinder ReleaseUploaderFinder
	Logger                *log.Logger
}

func (command UploadRelease) Run(_ []string) error {
	releaseSource, err := command.ReleaseUploaderFinder(command.TargetSourceID)
	if err != nil {
		return fmt.Errorf("error finding release source: %w", err)
	}

	file, err := command.FS.Open(command.LocalPath)
	if err != nil {
		return fmt.Errorf("could not open release: %w", err)
	}

	manifestReader := builder.NewReleaseManifestReader(command.FS)
	part, err := manifestReader.Read(command.LocalPath)
	if err != nil {
		return fmt.Errorf("error reading the release manifest: %w", err)
	}

	manifest := part.Metadata.(builder.ReleaseManifest)

	requirement := release.Requirement{Name: manifest.Name, Version: manifest.Version}
	_, found, err := releaseSource.GetMatchedRelease(requirement)
	if err != nil {
		return fmt.Errorf("couldn't query release source: %w", err)
	}

	if found {
		return fmt.Errorf("a release with name %q and version %q already exists on %s",
			manifest.Name, manifest.Version, command.TargetSourceID)
	}

	err = releaseSource.UploadRelease(manifest.Name, manifest.Version, file)
	if err != nil {
		return fmt.Errorf("error uploading the release: %w", err)
	}

	command.Logger.Println("Upload succeeded")

	return nil
}

