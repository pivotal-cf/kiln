package scenario

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func iRemoveAllTheObjectsInBucket(ctx context.Context, bucket string) error {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion()))
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(cfg)

	objectPaginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})

	var objectIds []s3types.ObjectIdentifier
	var listErr error

	for objectPaginator.HasMorePages() {
		page, err := objectPaginator.NextPage(ctx)
		if err != nil {
			var noBucket *s3types.NoSuchBucket
			if errors.As(err, &noBucket) {
				fmt.Printf("Bucket %s does not exist.\n", bucket)
				listErr = noBucket
			} else {
				listErr = err
			}

			break
		}

		for _, o := range page.Contents {
			fmt.Printf("      deleting %s\n", *o.Key)
			objectIds = append(objectIds, s3types.ObjectIdentifier{Key: o.Key})
		}
	}

	var deleteErr error
	output, deleteErr := s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3types.Delete{Objects: objectIds, Quiet: aws.Bool(true)},
	})
	if err != nil || len(output.Errors) > 0 {
		fmt.Printf("Error deleting objects from bucket %s.\n", bucket)
		if err != nil {
			var noBucket *s3types.NoSuchBucket
			if errors.As(err, &noBucket) {
				fmt.Printf("Bucket %s does not exist.\n", bucket)
				deleteErr = noBucket
			} else {
				deleteErr = err
			}
		} else if len(output.Errors) > 0 {
			for _, outErr := range output.Errors {
				fmt.Printf("%s: %s\n", *outErr.Key, *outErr.Message)
			}
			deleteErr = fmt.Errorf("%s", *output.Errors[0].Message)
		}
	} else {
		for _, delObjs := range output.Deleted {
			err = s3.NewObjectNotExistsWaiter(s3Client).Wait(
				ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: delObjs.Key}, time.Minute)
			if err != nil {
				fmt.Printf("Failed attempt to wait for object %s to be deleted.\n", *delObjs.Key)
			}
		}
	}

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
