package baking

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pivotal-cf/kiln/builder"
	yaml "gopkg.in/yaml.v2"
)

type StemcellService struct {
	logger        logger
	tarballReader partReader
}

func NewStemcellService(logger logger, tarballReader partReader) StemcellService {
	return StemcellService{
		logger:        logger,
		tarballReader: tarballReader,
	}
}

func (ss StemcellService) FromDirectories(directories []string) (stemcell map[string]interface{}, err error) {
	ss.logger.Println("Reading stemcells from directories...")

	var tarballs []string
	for _, directory := range directories {
		err := filepath.Walk(directory, filepath.WalkFunc(func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if match, _ := regexp.MatchString("tgz$|tar.gz$", path); match {
				tarballs = append(tarballs, path)
			}

			return nil
		}))

		if err != nil {
			return nil, err
		}
	}

	manifests := map[string]interface{}{}
	for _, tarball := range tarballs {
		manifest, err := ss.tarballReader.Read(tarball)
		if err != nil {
			return nil, err
		}
		stemcell := manifest.Metadata.(builder.StemcellManifest)
		_, ok := manifests[stemcell.OperatingSystem]
		if ok {
			errorMsg := fmt.Sprintf("more than one OS version was found for OS '%s' when parsing stemcells", stemcell.OperatingSystem)
			return nil, errors.New(errorMsg)
		}

		manifests[stemcell.OperatingSystem] = manifest.Metadata
	}

	return manifests, nil
}

func (ss StemcellService) FromTarball(path string) (interface{}, error) {
	if path == "" {
		return nil, nil
	}

	ss.logger.Println("Reading stemcell manifest...")

	stemcell, err := ss.tarballReader.Read(path)
	if err != nil {
		return nil, err
	}

	return stemcell.Metadata, nil
}

func (ss StemcellService) FromAssetsFile(assetsFilePath string) (map[string]interface{}, error) {
	assetsLockFilePath := fmt.Sprintf("%s.lock", strings.TrimSuffix(assetsFilePath, filepath.Ext(assetsFilePath)))
	assetsLockBasename := path.Base(assetsLockFilePath)
	ss.logger.Println(fmt.Sprintf("Reading stemcell criteria from %s", assetsLockBasename))
	assetsLockFile, err := os.Open(assetsLockFilePath)
	if err != nil {
		return nil, err
	}

	type stemcellMetadata struct {
		Version         string `yaml:"version"`
		OperatingSystem string `yaml:"os"`
	}

	stemcellCriteria := struct {
		Metadata stemcellMetadata `yaml:"stemcell_criteria"`
	}{}

	lockFileContent, err := ioutil.ReadAll(assetsLockFile)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(lockFileContent, &stemcellCriteria)
	if err != nil {
		return nil, err
	}

	stemcell := stemcellCriteria.Metadata

	stemcellManifest := map[string]interface{}{
		stemcell.OperatingSystem: stemcellCriteria.Metadata,
	}

	return stemcellManifest, err
}
