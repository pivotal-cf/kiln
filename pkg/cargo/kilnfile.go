package cargo

import "github.com/pivotal-cf/kiln/internal/component"

type Kilnfile struct {
	MajorVersion    int                       `yaml:"kiln_major_version,omitempty"`
	ReleaseSources  *component.ReleaseSources `yaml:"release_sources,omitempty"`
	Slug            string                    `yaml:"slug,omitempty"`
	PreGaUserGroups []string                  `yaml:"pre_ga_user_groups,omitempty"`
	Releases        []ReleaseSpec             `yaml:"releases,omitempty"`
	TileNames       []string                  `yaml:"tile_names,omitempty"`
	Stemcell        Stemcell                  `yaml:"stemcell_criteria,omitempty"`
}

func (kf Kilnfile) FindReleaseWithName(name string) (ReleaseSpec, error) {
	for _, s := range kf.Releases {
		if s.Name == name {
			return s, nil
		}
	}
	return ReleaseSpec{}, errorSpecNotFound(name)
}

func (kf Kilnfile) UpdateReleaseWithName(name string, spec ReleaseSpec) error {
	for i, r := range kf.Releases {
		if r.Name == name {
			kf.Releases[i] = spec
			return nil
		}
	}
	return errorSpecNotFound(name)
}

type KilnfileLock struct {
	Releases []ReleaseLock `yaml:"releases"`
	Stemcell Stemcell      `yaml:"stemcell_criteria"`
}

func (k KilnfileLock) FindReleaseWithName(name string) (ReleaseLock, error) {
	for _, r := range k.Releases {
		if r.Name == name {
			return r, nil
		}
	}
	return ReleaseLock{}, errorSpecNotFound(name)
}

func (k KilnfileLock) UpdateReleaseWithName(name string, lock ReleaseLock) error {
	for i, r := range k.Releases {
		if r.Name == name {
			k.Releases[i] = lock
			return nil
		}
	}
	return errorSpecNotFound(name)
}
