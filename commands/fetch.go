package commands

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/pivotal-cf/jhanda"
	manifest "github.com/pivotal-cf/kiln/internal/cargo"
	yaml "gopkg.in/yaml.v2"
)

type Fetch struct {
	Options struct {
		assetsFile  string `short:"a" long:"assets-file" required:"true" description:"path to assets file"`
		releasesDir string `short:"rd" long:"releases-directory reqiored"true description:"path to directory to download releases into"`
	}
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

	for _, release := range releases {
		fmt.Sprintf("release: %s", release)
		fmt.Sprintf("stemcell: %s", stemcell)

		/*
			for each *tar.gz
				compare with regex
				if matched then compare with release.name/version stemcell.name/version
				if true download and end loop
				else continue
		*/

		// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
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
		Description:      "Fetches releases listed in assets file",
		ShortDescription: "fetches releases",
		Flags:            f.Options,
	}
}
