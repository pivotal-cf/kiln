package commands

import (
	"errors"
	"fmt"
	"log"
	"regexp"

	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4"
)

//go:generate counterfeiter -o ./fakes/s3_uploader.go --fake-name S3Uploader . S3Uploader

type S3Uploader interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

type UploadRelease struct {
	FS             billy.Filesystem
	KilnfileLoader KilnfileLoader
	UploaderConfig func(*cargo.ReleaseSourceConfig) S3Uploader
	Logger         *log.Logger

	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`

		ReleaseSource string `short:"rs" long:"release-source" required:"true" description:"name of the release source specified in the Kilnfile"`
		LocalPath     string `short:"lp" long:"local-path" required:"true" description:"path to BOSH release tarball"`
		RemotePath    string `short:"rp" long:"remote-path" required:"true" description:"path at the remote source"`
	}
}

func (uploadRelease UploadRelease) Execute(args []string) error {
	_, err := jhanda.Parse(&uploadRelease.Options, args)
	if err != nil {
		return err
	}

	kilnfile, _, err := uploadRelease.KilnfileLoader.LoadKilnfiles(
		uploadRelease.FS,
		uploadRelease.Options.Kilnfile,
		uploadRelease.Options.VariablesFiles,
		uploadRelease.Options.Variables,
	)
	if err != nil {
		return fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	file, err := uploadRelease.FS.Open(uploadRelease.Options.LocalPath)
	if err != nil {
		return fmt.Errorf("could not open release: %w", err)
	}

	var (
		rc *cargo.ReleaseSourceConfig

		validSourcesForErrOutput []string
	)

	for index, rel := range kilnfile.ReleaseSources {
		if rel.Type == fetcher.ReleaseSourceTypeS3 {
			validSourcesForErrOutput = append(validSourcesForErrOutput, rel.Bucket)
			if rel.Bucket == uploadRelease.Options.ReleaseSource {
				rc = &kilnfile.ReleaseSources[index]
				break
			}
		}
	}

	if rc == nil {
		const msg = "remote release source could not be found in Kilnfile (only release sources of type s3 are supported)"
		if len(validSourcesForErrOutput) > 0 {
			return fmt.Errorf(msg+", some acceptable sources are: %v", validSourcesForErrOutput)
		}
		return errors.New(msg)
	}

	re, err := regexp.Compile(rc.Regex)
	if err != nil {
		return fmt.Errorf("could not compile the regular expression in Kilnfile for %q: %w", rc.Bucket, err)
	}

	if !re.MatchString(uploadRelease.Options.RemotePath) {
		return fmt.Errorf("remote-path does not match regular expression in Kilnfile for %q", rc.Bucket)
	}

	uploader := uploadRelease.UploaderConfig(rc)

	if _, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(rc.Bucket),
		Key:    aws.String(uploadRelease.Options.RemotePath),
		Body:   file,
	}); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	uploadRelease.Logger.Println("upload succeeded")

	return nil
}

func (uploadRelease UploadRelease) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Uploads a BOSH Release to an S3 release source for use in kiln fetch",
		ShortDescription: "uploads a BOSH release to an s3 release_source",
		Flags:            uploadRelease.Options,
	}
}
