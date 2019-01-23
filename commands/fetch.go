package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/cargo"
	yaml "gopkg.in/yaml.v2"
)

const (
	ReleaseName     = "release_name"
	ReleaseVersion  = "release_version"
	StemcellOS      = "stemcell_os"
	StemcellVersion = "stemcell_version"
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

func ListObjects(bucket string, regex regexp.Regexp, s3Client s3iface.S3API) (map[cargo.CompiledRelease]string, error) {
	MatchedS3Objects := make(map[cargo.CompiledRelease]string)

	err := s3Client.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: aws.String(bucket),
		},
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			for _, s3Object := range page.Contents {
				if s3Object.Key == nil {
					continue
				}

				if !regex.MatchString(*s3Object.Key) {
					continue
				}

				matches := regex.FindStringSubmatch(*s3Object.Key)
				subgroup := make(map[string]string)
				for i, name := range regex.SubexpNames() {
					if i != 0 && name != "" {
						subgroup[name] = matches[i]
					}
				}

				MatchedS3Objects[cargo.CompiledRelease{
					Name:    subgroup[ReleaseName],
					Version: subgroup[ReleaseVersion],
					// StemcellOS:      subgroup[StemcellOS],
					StemcellVersion: subgroup[StemcellVersion],
				}] = *s3Object.Key
			}
			return true
		},
	)

	if err != nil {
		return nil, err
	}

	return MatchedS3Objects, nil
}

func DownloadReleases(releases []cargo.Release, stemcell cargo.Stemcell, bucket string, releasesDir string, matchedS3objects map[cargo.CompiledRelease]string, s3Client s3iface.S3API) error {
	downloader := s3manager.NewDownloaderWithClient(s3Client)

	for _, release := range releases {
		s3Key, ok := matchedS3objects[cargo.CompiledRelease{
			Name:    release.Name,
			Version: release.Version,
			// StemcellOS:      stemcell.OS,
			StemcellVersion: stemcell.Version,
		}]

		if !ok {
			return fmt.Errorf("Compiled release: %s, version: %s, stemcell OS: %s, stemcell version: %s, not found", release.Name, release.Version, stemcell.OS, stemcell.Version)
		}

		// outputFile := filepath.Join(f.ReleasesDir, fmt.Sprintf("%s-%s-%s-%s.tgz", release.Name, release.Version, stemcell.OS, stemcell.Version))
		outputFile := filepath.Join(releasesDir, fmt.Sprintf("%s-%s-%s.tgz", release.Name, release.Version, stemcell.Version))
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", outputFile, err)
		}

		fmt.Printf("downloading %s...\n", s3Key)
		_, err = downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3Key),
		})

		if err != nil {
			return fmt.Errorf("failed to download file, %v\n", err)
		}
	}

	return nil
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
	// TODO: Check the capture group names

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

	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(assets.CompiledReleases.Region),
		Credentials: credentials.NewStaticCredentials(assets.CompiledReleases.AccessKeyId, assets.CompiledReleases.SecretAccessKey, ""),
	}))
	s3Client := s3.New(sess)

	MatchedS3Objects, err := ListObjects(assets.CompiledReleases.Bucket, *regex, s3Client)
	if err != nil {
		return err
	}

	fmt.Printf("number of matched S3 objects: %d\n", len(MatchedS3Objects))

	releases := assetsLock.Releases
	stemcell := assetsLock.Stemcell

	DownloadReleases(releases, stemcell, assets.CompiledReleases.Bucket, f.ReleasesDir, MatchedS3Objects, s3Client)
	if err != nil {
		return err
	}

	return nil
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f,
	}
}
