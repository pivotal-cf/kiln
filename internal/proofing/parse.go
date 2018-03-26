package proofing

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

func Parse(path string) (ProductTemplate, error) {
	contents, err := ioutil.ReadFile(path)
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
