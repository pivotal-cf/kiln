package cargo

import (
	"fmt"
)

type Kilnfile struct {
	MajorVersion    int                   `yaml:"kiln_major_version,omitempty"`
	ReleaseSources  []ReleaseSourceConfig `yaml:"release_sources,omitempty"`
	Slug            string                `yaml:"slug,omitempty"`
	PreGaUserGroups []string              `yaml:"pre_ga_user_groups,omitempty"`
	Releases        []ComponentSpec       `yaml:"releases,omitempty"`
	TileNames       []string              `yaml:"tile_names,omitempty"`
	Stemcell        Stemcell              `yaml:"stemcell_criteria,omitempty"`
}

func (kf Kilnfile) FindReleaseWithName(name string) (ComponentSpec, error) {
	for _, s := range kf.Releases {
		if s.Name == name {
			return s, nil
		}
	}
	return ComponentSpec{}, errorSpecNotFound(name)
}

func (kf Kilnfile) UpdateReleaseWithName(name string, spec ComponentSpec) error {
	for i, r := range kf.Releases {
		if r.Name == name {
			kf.Releases[i] = spec
			return nil
		}
	}
	return errorSpecNotFound(name)
}

type KilnfileLock struct {
	Releases []ComponentLock `yaml:"releases"`
	Stemcell Stemcell        `yaml:"stemcell_criteria"`
}

func (k KilnfileLock) FindReleaseWithName(name string) (ComponentLock, error) {
	for _, r := range k.Releases {
		if r.Name == name {
			return r, nil
		}
	}
	return ComponentLock{}, errorSpecNotFound(name)
}

func (k KilnfileLock) UpdateReleaseWithName(name string, lock ComponentLock) error {
	for i, r := range k.Releases {
		if r.Name == name {
			k.Releases[i] = lock
			return nil
		}
	}
	return errorSpecNotFound(name)
}

const (
	ErrStemcellOSInfoMustBeValid = "stemcell os information is missing or invalid"
)

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

func errorSpecNotFound(name string) error {
	return fmt.Errorf("failed to find release with name %q", name)
}
