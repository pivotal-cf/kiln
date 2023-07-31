package builder

import (
	"bytes"
	"fmt"

	"github.com/crhntr/yamlutil/yamlnode"
	"gopkg.in/yaml.v3"
)

type KilnMetadata struct {
	MetadataGitSHA string `yaml:"metadata_git_sha,omitempty"`
	KilnVersion    string `yaml:"kiln_version,omitempty"`
}

func setKilnMetadata(in []byte, kilnMetadata KilnMetadata) ([]byte, error) {
	var productTemplate yaml.Node
	err := yaml.Unmarshal(in, &productTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse product template: %w", err)
	}

	_, hasMetadataVersionKey := yamlnode.LookupKey(&productTemplate, "metadata_version")
	if !hasMetadataVersionKey {
		return in, nil
	}

	kilnMetadataValueNode, fieldExists := yamlnode.LookupKey(&productTemplate, "kiln_metadata")
	if fieldExists {
		fmt.Println("WARNING: kiln_metadata is not set by kiln please remove it from your product template")
		if err := kilnMetadataValueNode.Encode(kilnMetadata); err != nil {
			return nil, err
		}
	} else {
		var productTemplatePartial yaml.Node
		if err := productTemplatePartial.Encode(struct {
			KilnMetadata KilnMetadata `yaml:"kiln_metadata"`
		}{
			KilnMetadata: kilnMetadata,
		}); err != nil {
			return nil, fmt.Errorf("failed to encode kiln_metadata: %w", err)
		}
		productTemplate.Content[0].Content = append(productTemplate.Content[0].Content, productTemplatePartial.Content...)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err = enc.Encode(productTemplate.Content[0])
	if err != nil {
		return nil, fmt.Errorf("failed to encode product template: %w", err)
	}
	return buf.Bytes(), nil
}
