package internal

import (
	"bytes"
	"fmt"
	"io"

	"github.com/pivotal-cf/om/config"
	"gopkg.in/yaml.v2"
)

// Things I don't like
// Arguments:
// - additionalProperties is JSON, decoded into a map
// additionalProperties is really only additionalProductProperties, and gets merged with configFile's productProperties section
// Starts with an io.Reader to a YAML structured config file, returns a JSON structured string/[]byte containing the added product properties

// ProductConfiguration is a data structure for configuring a tile.
// Formerly, we relied on om/config.ProductConfiguration for marshalling/unmarshalling
// the product configuration file. Unfortunately, they now set product-properties to omit-empty,
// which causes Ops Manifest to error when product-properties is set to an empty hash.
//
// To work around this, we now have our own copy of ProductConfiguration without omit-empty on product-properties
type ProductConfiguration struct {
	ProductName              string                           `yaml:"product-name,omitempty"`
	ProductProperties        map[string]any                   `yaml:"product-properties"` // remove omitempty
	NetworkProperties        map[string]any                   `yaml:"network-properties,omitempty"`
	ResourceConfigProperties map[string]config.ResourceConfig `yaml:"resource-config,omitempty"`
	ErrandConfigs            map[string]config.ErrandConfig   `yaml:"errand-config,omitempty"`
	SyslogProperties         map[string]any                   `yaml:"syslog-properties,omitempty"`
}

// Force our "fork" of ProductConfiguration to have the same fields as the version in om/config
var _ = config.ProductConfiguration(ProductConfiguration{})

// MergeAdditionalProductProperties takes product properties from the provided reader and merges them with data from the
// additionalProperties parameter. It also does some validation to ensure required fields are set.
func MergeAdditionalProductProperties(configFile io.Reader, additionalProperties map[string]any) (io.Reader, error) {
	yamlInput, err := io.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	var inputConfig ProductConfiguration
	err = yaml.Unmarshal(yamlInput, &inputConfig)
	if err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	if inputConfig.NetworkProperties == nil {
		return nil, fmt.Errorf("network properties must be provided in the config file")
	}

	if inputConfig.ProductProperties == nil {
		return nil, fmt.Errorf("product properties must be provided in the config file")
	}

	inputConfig.ProductProperties = mergeProperties(inputConfig.ProductProperties, additionalProperties)

	modifiedConfigFile := bytes.NewBufferString("")
	err = yaml.NewEncoder(modifiedConfigFile).Encode(&inputConfig)
	if err != nil {
		return nil, err
	}

	return modifiedConfigFile, nil
}

func mergeProperties(minimalProperties, additionalProperties map[string]any) map[string]any {
	combinedProperties := make(map[string]any, len(minimalProperties)+len(additionalProperties))
	for k, v := range minimalProperties {
		combinedProperties[k] = v
	}

	for k, v := range additionalProperties {
		combinedProperties[k] = map[string]any{
			"value": v,
		}
	}
	return combinedProperties
}
