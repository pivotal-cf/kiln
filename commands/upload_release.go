package commands

import (
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/release"
	"io"
	"log"

	"github.com/pivotal-cf/kiln/builder"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4"
)

//go:generate counterfeiter -o ./fakes/release_uploader.go --fake-name ReleaseUploader . ReleaseUploader
type ReleaseUploader interface {
	UploadRelease(name, version string, file io.Reader) error
	fetcher.ReleaseSource
}

type UploadRelease struct {
	FS                    billy.Filesystem
	KilnfileLoader        KilnfileLoader
	ReleaseSourcesFactory ReleaseSourcesFactory
	Logger                *log.Logger

	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`

		ReleaseSource string `short:"rs" long:"release-source" required:"true" description:"name of the release source specified in the Kilnfile"`
		LocalPath     string `short:"lp" long:"local-path" required:"true" description:"path to BOSH release tarball"`
	}
}

func (command UploadRelease) Execute(args []string) error {
	_, err := jhanda.Parse(&command.Options, args)
	if err != nil {
		return err
	}

	uploader, err := command.findUploader()
	if err != nil {
		return err
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

	err = ensureNoExistingRelease(manifest.Name, manifest.Version, uploader)
	if err != nil {
		return err
	}

	err = uploader.UploadRelease(manifest.Name, manifest.Version, file)
	if err != nil {
		return fmt.Errorf("error uploading the release: %w", err)
	}

	command.Logger.Println("Upload succeeded")

	return nil
}

func ensureNoExistingRelease(name, version string, uploader ReleaseUploader) error {
	requirement := release.ReleaseRequirement{Name: name, Version: version}
	_, found, err := uploader.GetMatchedRelease(requirement)
	if err != nil {
		return fmt.Errorf("couldn't query release source: %w", err)
	}

	if found {
		return fmt.Errorf("a release with name %q and version %q already exists on %s",
			name, version, uploader.ID())
	}

	return nil
}

func (command UploadRelease) findUploader() (ReleaseUploader, error) {
	kilnfile, _, err := command.KilnfileLoader.LoadKilnfiles(
		command.FS,
		command.Options.Kilnfile,
		command.Options.VariablesFiles,
		command.Options.Variables,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	releaseSources := command.ReleaseSourcesFactory.ReleaseSources(kilnfile, false)

	var uploaderIDs []string

	for _, source := range releaseSources {
		u, ok := source.(ReleaseUploader)
		if ok {
			uploaderIDs = append(uploaderIDs, u.ID())
			if u.ID() == command.Options.ReleaseSource {
				return u, nil
			}
		}
	}

	if len(uploaderIDs) > 0 {
		return nil, fmt.Errorf(
			"could not find a valid matching release source in the Kilnfile, available upload-compatible sources are: %v",
			uploaderIDs,
		)
	}
	return nil, errors.New("no upload-capable release sources were found in the Kilnfile")
}

func (command UploadRelease) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Uploads a BOSH Release to an S3 release source for use in kiln fetch",
		ShortDescription: "uploads a BOSH release to an s3 release_source",
		Flags:            command.Options,
	}
}
