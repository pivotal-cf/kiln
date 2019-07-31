package fetcher

import (
	"io"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

//go:generate counterfeiter -o ./fakes/s3_downloader.go --fake-name S3Downloader . S3Downloader
type S3Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

//go:generate counterfeiter -o ./fakes/s3_object_lister.go --fake-name S3ObjectLister . S3ObjectLister
type S3ObjectLister interface {
	ListObjectsPages(*s3.ListObjectsInput, func(*s3.ListObjectsOutput, bool) bool) error
}

type S3ReleaseSource struct {
	Logger       *log.Logger
	S3Client     S3ObjectLister
	S3Downloader S3Downloader
	Bucket       string
	Regex        string
}

func (r *S3ReleaseSource) Configure(config cargo.S3ReleaseConfig) {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(config.Region),
		Credentials: credentials.NewStaticCredentials(
			config.AccessKeyId,
			config.SecretAccessKey,
			"",
		),
	}))
	client := s3.New(sess)

	r.S3Client = client
	r.S3Downloader = s3manager.NewDownloaderWithClient(client)

	r.Bucket = config.Bucket
	r.Regex = config.Regex
}
