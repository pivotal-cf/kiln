package internal

import (
	"encoding/json"
	"fmt"
	"strings"
)

type OMRunner struct {
	cmdRunner CommandRunner
	FileIO    FileIO
}

type StagedProduct struct {
	GUID           string `json:"guid"`
	Type           string `json:"type"`
	ProductVersion string `json:"product_version"`
}

type stagedManifestResponse struct {
	Manifest map[string]interface{}
	Errors   OMError `json:"errors"`
}

type OMError struct {
	// XXX: reconsider, the key here may change depending on the endpoint
	Messages []string `json:"base"`
}

func NewOMRunner(cmdRunner CommandRunner, fileIO FileIO) OMRunner {
	return OMRunner{
		cmdRunner: cmdRunner,
		FileIO:    fileIO,
	}
}

func (o OMRunner) StagedProducts() ([]StagedProduct, error) {
	response, errOutput, err := o.cmdRunner.Run(
		"om",
		"--skip-ssl-validation",
		"curl",
		"--path", "/api/v0/staged/products",
	)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve staged products: %s: %s", err, errOutput)
	}

	var stagedProducts []StagedProduct
	err = json.Unmarshal([]byte(response), &stagedProducts)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve staged products: %s", err)
	}

	return stagedProducts, nil
}

func (o OMRunner) FindStagedProduct(productName string) (StagedProduct, error) {
	stagedProducts, _ := o.StagedProducts()

	var stagedTypes []string
	for _, sp := range stagedProducts {
		if sp.Type == productName {
			return sp, nil
		} else {
			stagedTypes = append(stagedTypes, sp.Type)
		}
	}

	return StagedProduct{}, fmt.Errorf("Product %q has not been staged. Staged products: %q",
		productName, strings.Join(stagedTypes, ", "))
}

func (o OMRunner) ResetAndConfigure(productName string, productVersion string, configJSON string) error {
	_, errOutput, err := o.cmdRunner.Run(
		"om",
		"--skip-ssl-validation",
		"curl",
		"-x", "DELETE",
		"--path", "/api/v0/staged",
	)

	if err != nil {
		return fmt.Errorf("Unable to revert staged changes: %s: %s", err, errOutput)
	}

	_, errOutput, err = o.cmdRunner.Run(
		"om",
		"--skip-ssl-validation",
		"stage-product",
		"--product-name", productName,
		"--product-version", productVersion,
	)

	if err != nil {
		return fmt.Errorf("Unable to stage product %q, version %q: %s: %s",
			productName, productVersion, err, errOutput)
	}

	configFile, err := o.FileIO.TempFile("", "")
	if err != nil {
		return fmt.Errorf("Unable to ResetAndConfigure: %s", err)
	}
	defer o.FileIO.Remove(configFile.Name())

	_, err = configFile.WriteString(configJSON)
	if err != nil {
		return err // un-tested
	}

	_, errOutput, err = o.cmdRunner.Run(
		"om",
		"--skip-ssl-validation",
		"configure-product",
		"--config", configFile.Name(),
	)

	if err != nil {
		return fmt.Errorf("Unable to configure product %q: %s: %s", productName, err, errOutput)
	}

	return nil
}

func (o OMRunner) GetManifest(productGUID string) (map[string]interface{}, error) {
	response, errOutput, err := o.cmdRunner.Run(
		"om",
		"--skip-ssl-validation",
		"curl",
		"--path", fmt.Sprintf("/api/v0/staged/products/%s/manifest", productGUID),
	)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve staged manifest for product guid %q: %s: %s", productGUID, err, errOutput)
	}
	var smr stagedManifestResponse
	err = json.Unmarshal([]byte(response), &smr)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve staged manifest for product guid %q: %s", productGUID, err)
	}
	if len(smr.Errors.Messages) > 0 {
		return nil, fmt.Errorf("Unable to retrieve staged manifest for product guid %q: %s",
			productGUID,
			smr.Errors.Messages[0])
	}

	return smr.Manifest, nil
}
