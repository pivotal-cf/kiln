package commands

import (
	"fmt"
	"io"
	"log"
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
	logger *log.Logger

	Options struct {
		AssetsFile  string `short:"a" long:"assets-file" required:"true" description:"path to assets file"`
		ReleasesDir string `short:"rd" long:"releases-directory" required:"true" description:"path to a directory to download releases into"`
	}
}

func NewFetch(logger *log.Logger) Fetch {
	return Fetch{
		logger: logger,
	}
}

type CompiledReleasesRegexp struct {
	r *regexp.Regexp
}

func NewCompiledReleasesRegexp(regex string) (*CompiledReleasesRegexp, error) {
	r, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}

	var count int
	for _, name := range r.SubexpNames() {
		if name == ReleaseName || name == ReleaseVersion || name == StemcellOS || name == StemcellVersion {
			count++
		}
	}
	if count != 4 {
		return nil, fmt.Errorf("Missing some capture group. Required capture groups: %s, %s, %s, %s", ReleaseName, ReleaseVersion, StemcellOS, StemcellVersion)
	}

	return &CompiledReleasesRegexp{r: r}, nil
}

func (crr *CompiledReleasesRegexp) Convert(s3Key string) (cargo.CompiledRelease, error) {
	if !crr.r.MatchString(s3Key) {
		return cargo.CompiledRelease{}, fmt.Errorf("s3 key does not match regex")
	}

	matches := crr.r.FindStringSubmatch(s3Key)
	subgroup := make(map[string]string)
	for i, name := range crr.r.SubexpNames() {
		if i != 0 && name != "" {
			subgroup[name] = matches[i]
		}
	}

	return cargo.CompiledRelease{
		Name:            subgroup[ReleaseName],
		Version:         subgroup[ReleaseVersion],
		StemcellOS:      subgroup[StemcellOS],
		StemcellVersion: subgroup[StemcellVersion],
	}, nil
}

//go:generate counterfeiter -o ./fakes/s3client.go --fake-name S3Client github.com/pivotal-cf/kiln/vendor/github.com/aws/aws-sdk-go/service/s3/s3iface.S3API
func ListObjects(bucket string, regex *CompiledReleasesRegexp, s3Client s3iface.S3API) (map[cargo.CompiledRelease]string, error) {
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

				compiledRelease, err := regex.Convert(*s3Object.Key)
				if err != nil {
					continue
				}

				MatchedS3Objects[compiledRelease] = *s3Object.Key
			}
			return true
		},
	)

	if err != nil {
		return nil, err
	}

	return MatchedS3Objects, nil
}

//go:generate counterfeiter -o ./fakes/downloader.go --fake-name Downloader . Downloader
type Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

func DownloadReleases(logger *log.Logger, assetsLock cargo.AssetsLock, bucket string, matchedS3Objects map[cargo.CompiledRelease]string, fileCreator func(string) (io.WriterAt, error), downloader Downloader) error {
	releases := assetsLock.Releases
	stemcell := assetsLock.Stemcell

	for _, release := range releases {
		s3Key, ok := matchedS3Objects[cargo.CompiledRelease{
			Name:            release.Name,
			Version:         release.Version,
			StemcellOS:      stemcell.OS,
			StemcellVersion: stemcell.Version,
		}]

		if !ok {
			return fmt.Errorf("Compiled release: %s, version: %s, stemcell OS: %s, stemcell version: %s, not found", release.Name, release.Version, stemcell.OS, stemcell.Version)
		}

		outputFile := fmt.Sprintf("%s-%s-%s-%s.tgz", release.Name, release.Version, stemcell.OS, stemcell.Version)
		file, err := fileCreator(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", outputFile, err)
		}

		logger.Printf("downloading %s...\n", s3Key)
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
	args, err := jhanda.Parse(&f.Options, args)
	if err != nil {
		return err
	}

	f.logger.Println("getting S3 information from assets.yml")
	file, err := os.Open(f.Options.AssetsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var assets cargo.Assets
	err = yaml.NewDecoder(file).Decode(&assets)
	if err != nil {
		return err
	}

	compiledRegex, err := NewCompiledReleasesRegexp(assets.CompiledReleases.Regex)
	if err != nil {
		return err
	}

	f.logger.Println("getting release information from assets.lock")
	assetsLockFile, err := os.Open(fmt.Sprintf("%s.lock", strings.TrimSuffix(f.Options.AssetsFile, ".yml")))
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

	MatchedS3Objects, err := ListObjects(assets.CompiledReleases.Bucket, compiledRegex, s3Client)
	if err != nil {
		return err
	}

	f.logger.Printf("number of matched S3 objects: %d\n", len(MatchedS3Objects))

	fileCreator := func(filename string) (io.WriterAt, error) {
		return os.Create(filepath.Join(f.Options.ReleasesDir, filename))
	}

	downloader := s3manager.NewDownloaderWithClient(s3Client)
	return DownloadReleases(f.logger, assetsLock, assets.CompiledReleases.Bucket, MatchedS3Objects, fileCreator, downloader)
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
