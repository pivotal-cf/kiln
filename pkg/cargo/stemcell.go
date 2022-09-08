package cargo

import "fmt"

type Stemcell struct {
	Alias   string `yaml:"alias,omitempty"`
	OS      string `yaml:"os"`
	Version string `yaml:"version"`

	// Slug is the TanzuNetwork product slug
	// it is used to find new stemcell versions
	Slug string `json:"slug,omitempty"`
}

func (s Stemcell) ProductSlug() (string, error) {
	if s.Slug != "" {
		return s.Slug, nil
	}
	switch s.OS {
	case "ubuntu-xenial":
		return "stemcells-ubuntu-xenial", nil
	case "windows2019":
		return "stemcells-windows-server", nil
	case "ubuntu-jammy":
		return "stemcells-ubuntu-jammy", nil
	default:
		return "", fmt.Errorf("%s: .stemcell.slug not set in Kilnfile", ErrStemcellOSInfoMustBeValid)
	}
}
