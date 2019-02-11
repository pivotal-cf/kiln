package baking

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
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

func (ss StemcellService) FromAssetsFile(assetsFilePath string) (interface{}, error) {
	var stemcellManifest builder.StemcellManifest
	assetsLockFilePath := fmt.Sprintf("%s.lock", strings.TrimSuffix(assetsFilePath, ".yml"))
	assetsLockBasename := path.Base(assetsLockFilePath)
	ss.logger.Println(fmt.Sprintf("Reading stemcell criteria from %s", assetsLockBasename))
	assetsLockFile, err := os.Open(assetsLockFilePath)
	if err != nil {
		return stemcellManifest, err
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
		return stemcellCriteria.Metadata, err
	}

	err = yaml.Unmarshal(lockFileContent, &stemcellCriteria)
	if err != nil {
		return stemcellCriteria.Metadata, err
	}

	return stemcellCriteria.Metadata, err
}
