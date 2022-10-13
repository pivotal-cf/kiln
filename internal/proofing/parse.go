package proofing

import (
	"io"

	"gopkg.in/yaml.v2"
)

func Parse(r io.Reader) (ProductTemplate, error) {
	contents, err := io.ReadAll(r)
	if err != nil {
		return ProductTemplate{}, err
	}

	var productTemplate ProductTemplate
	err = yaml.Unmarshal(contents, &productTemplate)
	if err != nil {
		return ProductTemplate{}, err
	}

	return productTemplate, nil
}
