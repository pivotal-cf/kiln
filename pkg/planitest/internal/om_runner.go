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
	Manifest map[string]any
	Errors   OMError `json:"errors"`
}

type OMError struct {
	// Messages: reconsider, the key here may change depending on the endpoint
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
		return nil, fmt.Errorf("unable to retrieve staged products: %w: %s", err, errOutput)
	}

	var stagedProducts []StagedProduct
	err = json.Unmarshal([]byte(response), &stagedProducts)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve staged products: %w", err)
	}

	return stagedProducts, nil
}

func (o OMRunner) FindStagedProduct(productName string) (StagedProduct, error) {
	stagedProducts, err := o.StagedProducts()
	if err != nil {
		return StagedProduct{}, err
	}

	var stagedTypes []string
	for _, sp := range stagedProducts {
		if sp.Type == productName {
			return sp, nil
		} else {
			stagedTypes = append(stagedTypes, sp.Type)
		}
	}

	return StagedProduct{}, fmt.Errorf("product %q has not been staged. Staged products: %q",
		productName, strings.Join(stagedTypes, ", "))
}

func (o OMRunner) ResetAndConfigure(productName string, productVersion string, configJSON string) error {
	// Re-staging churns Ops Manager's generated properties (certs, keys) for every
	// product staged alongside productName, which can dirty unrelated dependency
	// products. If productName is already staged at the requested version, skip
	// straight to configure-product instead of unstaging and re-staging it.
	stagedProduct, err := o.FindStagedProduct(productName)
	alreadyStagedAtVersion := err == nil && stagedProduct.ProductVersion == productVersion

	if !alreadyStagedAtVersion {
		if err == nil {
			// It is staged at a different version. We should revert and unstage.
			_, errOutput, err := o.cmdRunner.Run(
				"om",
				"--skip-ssl-validation",
				"revert-staged-changes",
			)
			if err != nil {
				return fmt.Errorf("unable to revert staged changes: %w: %s", err, errOutput)
			}

			// Unstage the product if it's already staged. Ignore errors if it's not staged.
			_, _, _ = o.cmdRunner.Run(
				"om",
				"--skip-ssl-validation",
				"unstage-product",
				"--product-name", productName,
			)
		}

		_, errOutput, err := o.cmdRunner.Run(
			"om",
			"--skip-ssl-validation",
			"stage-product",
			"--product-name", productName,
			"--product-version", productVersion,
		)
		if err != nil {
			return fmt.Errorf("unable to stage product %q, version %q: %w: %s",
				productName, productVersion, err, errOutput)
		}
	}

	configFile, err := o.FileIO.TempFile("", "")
	if err != nil {
		return fmt.Errorf("unable to ResetAndConfigure: %w", err)
	}
	defer func() {
		_ = o.FileIO.Remove(configFile.Name())
	}()

	_, err = configFile.WriteString(configJSON)
	if err != nil {
		return err // un-tested
	}

	_, errOutput, err := o.cmdRunner.Run(
		"om",
		"--skip-ssl-validation",
		"configure-product",
		"--config", configFile.Name(),
	)
	if err != nil {
		return fmt.Errorf("unable to configure product %q: %w: %s", productName, err, errOutput)
	}

	return nil
}

func (o OMRunner) GetManifest(productGUID string) (map[string]any, error) {
	response, errOutput, err := o.cmdRunner.Run(
		"om",
		"--skip-ssl-validation",
		"curl",
		"--path", fmt.Sprintf("/api/v0/staged/products/%s/manifest", productGUID),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve staged manifest for product guid %q: %w: %s", productGUID, err, errOutput)
	}
	var smr stagedManifestResponse
	err = json.Unmarshal([]byte(response), &smr)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve staged manifest for product guid %q: %w", productGUID, err)
	}
	if len(smr.Errors.Messages) > 0 {
		return nil, fmt.Errorf("unable to retrieve staged manifest for product guid %q: %s",
			productGUID,
			smr.Errors.Messages[0])
	}

	return smr.Manifest, nil
}
