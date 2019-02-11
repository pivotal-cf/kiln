package commands

import (
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
	yaml "gopkg.in/yaml.v2"
)

const (
	ReleaseName     = "release_name"
	ReleaseVersion  = "release_version"
	StemcellOS      = "stemcell_os"
	StemcellVersion = "stemcell_version"
)

func S3ClientProvider(session *session.Session, cfgs ...*aws.Config) s3iface.S3API {
	return s3.New(session, cfgs...)
}

type Fetch struct {
	logger *log.Logger

	S3Provider func(*session.Session, ...*aws.Config) s3iface.S3API

	Options struct {
		AssetsFile      string   `short:"a" long:"assets-file" required:"true" description:"path to assets file"`
		VariablesFiles  []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables       []string `short:"vr" long:"variable" description:"variable in key=value format"`
		ReleasesDir     string   `short:"rd" long:"releases-directory" required:"true" description:"path to a directory to download releases into"`
		DownloadThreads int      `short:"dt" long:"download-threads" description:"number of parallel threads to download parts from S3"`
	}
}

func NewFetch(logger *log.Logger, s3Provider func(*session.Session, ...*aws.Config) s3iface.S3API) Fetch {
	return Fetch{
		logger:     logger,
		S3Provider: s3Provider,
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
func GetMatchedReleases(bucket string, regex *CompiledReleasesRegexp, s3Client s3iface.S3API, assetsLock cargo.AssetsLock) (map[cargo.CompiledRelease]string, error) {
	matchedS3Objects := make(map[cargo.CompiledRelease]string)

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

				matchedS3Objects[compiledRelease] = *s3Object.Key
			}
			return true
		},
	)
	if err != nil {
		return nil, err
	}

	missingReleases := make([]cargo.CompiledRelease, 0)
	for _, release := range assetsLock.Releases {
		expectedRelease := cargo.CompiledRelease{
			Name:            release.Name,
			Version:         release.Version,
			StemcellOS:      assetsLock.Stemcell.OS,
			StemcellVersion: assetsLock.Stemcell.Version,
		}
		_, ok := matchedS3Objects[expectedRelease]

		if !ok {
			missingReleases = append(missingReleases, expectedRelease)
		}
	}
	if len(missingReleases) > 0 {
		formattedMissingReleases := make([]string, 0)

		for _, missingRelease := range missingReleases {
			formattedMissingReleases = append(formattedMissingReleases, fmt.Sprintf(
				"%+v", missingRelease,
			))

		}
		return nil, fmt.Errorf("Expected releases were not matched by the regex:\n%s", strings.Join(formattedMissingReleases, "\n"))
	}

	return matchedS3Objects, nil
}

//go:generate counterfeiter -o ./fakes/downloader.go --fake-name Downloader . Downloader
type Downloader interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

func DownloadReleases(logger *log.Logger, assetsLock cargo.AssetsLock, bucket string, matchedS3Objects map[cargo.CompiledRelease]string, fileCreator func(string) (io.WriterAt, error), downloader Downloader, downloadThreads int) error {
	releases := assetsLock.Releases
	stemcell := assetsLock.Stemcell

	setConcurrency := func(dl *s3manager.Downloader) {
		if downloadThreads > 0 {
			dl.Concurrency = downloadThreads
		}
	}

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
		}, setConcurrency)

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

	templateVariablesService := baking.NewTemplateVariablesService()
	templateVariables, err := templateVariablesService.FromPathsAndPairs(f.Options.VariablesFiles, f.Options.Variables)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %s", err)
	}

	assetsYAML, err := ioutil.ReadFile(f.Options.AssetsFile)
	if err != nil {
		return err
	}

	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, assetsYAML)
	if err != nil {
		return err
	}

	f.logger.Println("getting S3 information from assets.yml")

	var assets cargo.Assets
	err = yaml.Unmarshal(interpolatedMetadata, &assets)
	if err != nil {
		return err
	}

	compiledRegex, err := NewCompiledReleasesRegexp(assets.CompiledReleases.Regex)
	if err != nil {
		return err
	}

	f.logger.Println("getting release information from assets.lock")
	assetsLockFile, err := os.Open(fmt.Sprintf("%s.lock", strings.TrimSuffix(f.Options.AssetsFile, filepath.Ext(f.Options.AssetsFile))))
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
	s3Client := f.S3Provider(sess)

	MatchedS3Objects, err := GetMatchedReleases(assets.CompiledReleases.Bucket, compiledRegex, s3Client, assetsLock)
	if err != nil {
		return err
	}

	f.logger.Printf("number of matched S3 objects: %d\n", len(MatchedS3Objects))

	fileCreator := func(filename string) (io.WriterAt, error) {
		return os.Create(filepath.Join(f.Options.ReleasesDir, filename))
	}

	downloader := s3manager.NewDownloaderWithClient(s3Client)
	return DownloadReleases(f.logger, assetsLock, assets.CompiledReleases.Bucket, MatchedS3Objects, fileCreator, downloader, f.Options.DownloadThreads)
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
