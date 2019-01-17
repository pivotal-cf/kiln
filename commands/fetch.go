package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/jhanda"
	manifest "github.com/pivotal-cf/kiln/internal/cargo"
	yaml "gopkg.in/yaml.v2"
)

type Fetch struct {
	Options struct {
		assetsFile  string `short:"a" long:"assets-file" required:"true" description:"path to assets file"`
		releasesDir string `short:"rd" long:"releases-directory" required:"true" description:"path to a directory to download releases into"`
	}
}

func NewFetch() Fetch {
	return Fetch{}
}

func (f Fetch) Execute(args []string) error {
	args, err := jhanda.Parse(&f.Options, args)
	if err != nil {
		return err
	}

	// get s3 bucket information from assets.yml
	data, err := ioutil.ReadFile(f.Options.assetsFile)
	if err != nil {
		return err
	}
	compiledReleasesBucket := &manifest.CompiledReleases{}
	err = yaml.Unmarshal([]byte(data), compiledReleasesBucket)

	// get release names and versions from assets.lock
	assetsLockFile := fmt.Sprintf("%s.lock", strings.TrimSuffix(f.Options.assetsFile, ".yml"))
	data, err = ioutil.ReadFile(assetsLockFile)
	if err != nil {
		return err
	}
	assetsLock := &manifest.AssetsLock{}
	err = yaml.Unmarshal([]byte(data), assetsLock)
	releases := assetsLock.Releases
	stemcell := assetsLock.Stemcell

	// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(compiledReleasesBucket.Region),
		Credentials: credentials.NewStaticCredentials(compiledReleasesBucket.AccessKeyId, compiledReleasesBucket.SecretAccessKey, ""),
	}))
	downloader := s3manager.NewDownloader(sess)

	for _, release := range releases {
		fmt.Sprintf("release: %s", release)
		fmt.Sprintf("stemcell: %s", stemcell)

		filename := fmt.Sprintf("2.5/%s/%s-%s.tgz", release.Name, release.Name, release.Version)
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", filename, err)
		}

		n, err := downloader.Download(file, &s3.GetObjectInput{
			Bucket: aws.String(compiledReleasesBucket.Bucket),
			Key:    aws.String(filename),
		})

		if err != nil {
			return fmt.Errorf("failed to download file, %v", err)
		}

		fmt.Printf("release downloaded, %d bytes\n", n)
		/*
			for each *tar.gz
				compare with regex
				if matched then compare with release.name/version stemcell.name/version
				if true download and end loop
				else continue
		*/

		// list bucket contents
		// aws s3 cp s3://compiled-releases/{s3.Regex} f.Options.releasesDir

		// ^2.5/<release_name>/(?P<release_name>[a-z]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-]+)-(?P<stemcell_version>[\d\.]+)\.tgz$
		// Need to figure out how to use compiledReleasesBucket.Regex with release.Name and release.Version
		// Have a separate compiledReleasesBucket.Path?
	}

	return nil
}

func (f Fetch) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
