package commands

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/cargo"
	yaml "gopkg.in/yaml.v2"
)

type Fetch struct {
	AssetsFile  string `short:"a" long:"assets-file" required:"true" description:"path to assets file"`
	ReleasesDir string `short:"rd" long:"releases-directory" required:"true" description:"path to a directory to download releases into"`
}

func NewFetch(assetsFile string, releasesDir string) Fetch {
	return Fetch{
		AssetsFile:  assetsFile,
		ReleasesDir: releasesDir,
	}
}

func (f Fetch) Execute(args []string) error {
	args, err := jhanda.Parse(&f, args)
	if err != nil {
		return err
	}

	fmt.Println("getting S3 information from assets.yml")
	file, err := os.Open(f.AssetsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var assets cargo.Assets
	err = yaml.NewDecoder(file).Decode(&assets)
	if err != nil {
		return err
	}

	regex, err := regexp.Compile(assets.CompiledReleases.Regex)
	if err != nil {
		return err
	}

	fmt.Println("getting release information from assets.lock")
	assetsLockFile, err := os.Open(fmt.Sprintf("%s.lock", strings.TrimSuffix(f.AssetsFile, ".yml")))
	if err != nil {
		return err
	}
	defer assetsLockFile.Close()

	var assetsLock cargo.AssetsLock
	err = yaml.NewDecoder(assetsLockFile).Decode(&assetsLock)
	if err != nil {
		return err
	}
	// releases := assetsLock.Releases
	// stemcell := assetsLock.Stemcell

	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(assets.CompiledReleases.Region),
		Credentials: credentials.NewStaticCredentials(assets.CompiledReleases.AccessKeyId, assets.CompiledReleases.SecretAccessKey, ""),
	}))
	s3Client := s3.New(sess)
	// downloader := s3manager.NewDownloaderWithClient(s3Svc)

	// fmt.Println("looping over all releases")
	// for _, release := range releases {
	// 	if err != nil {
	// 		return fmt.Errorf("failed to create filepath %v", err)
	// 	}
	// 	filename := fmt.Sprintf("2.5/%s/%s-%s-%s.tgz", release.Name, release.Name, release.Version, stemcell.Version)
	// 	outputFile := fmt.Sprintf("%s/%s-%s-%s.tgz", f.ReleasesDir, release.Name, release.Version, stemcell.Version)

	// 	file, err := os.Create(outputFile)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to create file %q, %v", outputFile, err)
	// 	}

	// 	fmt.Printf("downloading %s-%s-%s...\n", release.Name, release.Version, stemcell.Version)
	// 	fmt.Printf("s3 path: %s\n", filename)
	// 	n, err := downloader.Download(file, &s3.GetObjectInput{
	// 		Bucket: aws.String(compiledReleases.S3.Bucket),
	// 		Key:    aws.String(filename),
	// 	})

	// 	if err != nil {
	// 		return fmt.Errorf("failed to download file, %v\n", err)
	// 	}

	// 	fmt.Printf("release downloaded to %s directory, %d bytes\n", f.ReleasesDir, n)
	// }

	// return nil

	// list contents of bucket
	s3Client.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: aws.String(assets.CompiledReleases.Bucket),
		},
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			for _, s3Object := range page.Contents {
				if s3Object.Key == nil {
					continue
				}

				if !regex.MatchString(*s3Object.Key) {
					continue
				}

				regex.FindStringSubmatch(*s3Object.Key)
			}
			return false
		},
	)

	return nil
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f,
	}
}
