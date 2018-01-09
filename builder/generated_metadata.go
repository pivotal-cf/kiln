package builder

type GeneratedMetadata struct {
	Name             string
	StemcellCriteria StemcellCriteria
	Releases         []Release
	IconImage        string

	RuntimeConfigs []Part
	Variables      []Part

	Metadata Metadata
}

type Release struct {
	Name    string
	File    string
	Version string
}

type StemcellCriteria struct {
	Version string
	OS      string
}

func (gm GeneratedMetadata) MarshalYAML() (interface{}, error) {
	m := map[string]interface{}{}

	m["name"] = gm.Name

	if len(gm.RuntimeConfigs) > 0 {
		m["runtime_configs"] = gm.metadataOnly(gm.RuntimeConfigs)
	}
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
