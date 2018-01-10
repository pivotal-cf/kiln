package builder

type GeneratedMetadata struct {
	Name      string
	IconImage string

	Variables []Part

	Metadata Metadata
}

func (gm GeneratedMetadata) MarshalYAML() (interface{}, error) {
	m := map[string]interface{}{}

	m["name"] = gm.Name

	if len(gm.Variables) > 0 {
		m["variables"] = gm.metadataOnly(gm.Variables)
	}

	for k, v := range gm.Metadata {
		m[k] = v
	}

	return m, nil
}

func (gm GeneratedMetadata) metadataOnly(parts []Part) []interface{} {
	metadata := []interface{}{}
	for _, p := range parts {
		metadata = append(metadata, p.Metadata)
	}
	return metadata
}
