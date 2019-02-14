package providers

import (
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type S3Provider struct {
}

func NewS3Provider() S3Provider {
	return S3Provider{}
}

//go:generate counterfeiter -o ./fakes/s3_downloader.go --fake-name S3Downloader . S3Downloader
type S3Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

func (s S3Provider) GetS3Downloader(region, accessKeyID, secretAccessKey string) S3Downloader {
	return s3manager.NewDownloaderWithClient(s.GetS3Client(region, accessKeyID, secretAccessKey))
}

func (s S3Provider) GetS3Client(region, accessKeyID, secretAccessKey string) s3iface.S3API {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
	}))
	return s3.New(sess)
}
