package scenario

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func iRemoveAllTheObjectsInBucket(ctx context.Context, bucket string) error {
	awsSession, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(awsRegion()),
		},
	})
	if err != nil {
		return err
	}
	s3Session := s3.New(awsSession)

	var deleteErr error
	listErr := s3Session.ListObjectsV2PagesWithContext(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}, func(page *s3.ListObjectsV2Output, b bool) bool {
		var del s3.Delete
		for _, o := range page.Contents {
			fmt.Printf("      deleting %s\n", aws.StringValue(o.Key))
			del.Objects = append(del.Objects, &s3.ObjectIdentifier{
				Key: o.Key,
			})
		}
		_, deleteErr = s3Session.DeleteObjectsWithContext(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &del,
		})
		return deleteErr == nil
	})
	if deleteErr != nil {
		return deleteErr
	}
	return listErr
}

func awsRegion() string {
	region := os.Getenv("AWS_REGION")
	if region != "" {
		return region
	}
	defaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	if defaultRegion != "" {
		return defaultRegion
	}
	return "us-west-1"
}
