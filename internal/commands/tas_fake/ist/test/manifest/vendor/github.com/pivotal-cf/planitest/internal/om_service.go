package internal

import (
	"io"

	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type OMService struct {
	omRunner OMRunnerI
}

func NewOMServiceWithRunner(omRunner OMRunnerI) (*OMService, error) {
	return &OMService{omRunner: omRunner}, nil
}

func (o OMService) RenderManifest(tileConfig io.Reader, tileMetadata io.Reader) (string, error) {
	var m struct {
		Name           string `yaml:"name"`
		ProductVersion string `yaml:"product_version"`
	}
	err := yaml.NewDecoder(tileMetadata).Decode(&m)
	if err != nil {
		return "", err
	}

	stagedProduct, err := o.omRunner.FindStagedProduct(m.Name)
	if err != nil {
		return "", err
	}

	configInput, err := ioutil.ReadAll(tileConfig)
	if err != nil {
		return "", err
	}

	err = o.omRunner.ResetAndConfigure(m.Name, m.ProductVersion, string(configInput))
	if err != nil {
		return "", err
	}

	// calling configure can re-staged and update the product GUID
	stagedProduct, err = o.omRunner.FindStagedProduct(m.Name)
	if err != nil {
		return "", err
	}

	manifest, err := o.omRunner.GetManifest(stagedProduct.GUID)
	if err != nil {
		return "", err
	}

	y, err := yaml.Marshal(manifest)
	if err != nil {
		return "", err // un-tested
	}

	return string(y), nil
}
