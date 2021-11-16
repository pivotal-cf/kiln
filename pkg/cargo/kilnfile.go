package cargo

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver"
)

type KilnfileLock struct {
	Releases []ComponentLock `yaml:"releases"`
	Stemcell Stemcell        `yaml:"stemcell_criteria"`
}

type ComponentSpec struct {
	// Name is a required field and must be set with the bosh release name
	Name string `yaml:"name"`

	// Version if not set, it will default to ">0".
	// See https://github.com/Masterminds/semver for syntax
	Version string `yaml:"version,omitempty"`

	// StemcellOS may be set when a specifying a component
	// compiled with a particular stemcell. Usually you should
	// also set StemcellVersion when setting this field.
	StemcellOS string `yaml:"os,omitempty"`

	// StemcellVersion may be set when a specifying a component
	// compiled with a particular stemcell. Usually you should
	// also set StemcellOS when setting this field.
	StemcellVersion string `yaml:"stemcell_version,omitempty"`

	// Repositories are where the BOSH release source code is
	GitRepositories []string `yaml:"git_repos,omitempty"`
}

func (spec ComponentSpec) VersionConstraints() (*semver.Constraints, error) {
	if spec.Version == "" {
		spec.Version = ">0"
	}
	c, err := semver.NewConstraint(spec.Version)
	if err != nil {
		return nil, fmt.Errorf("expected version to be a Constraint: %w", err)
	}
	return c, nil
}

func (spec ComponentSpec) Lock() ComponentLock {
	return ComponentLock{
		Name:            spec.Name,
		Version:         spec.Version,
		StemcellOS:      spec.StemcellOS,
		StemcellVersion: spec.StemcellVersion,
	}
}

func (spec ComponentSpec) UnsetStemcell() ComponentSpec {
	spec.StemcellOS = ""
	spec.StemcellVersion = ""
	return spec
}

type Kilnfile struct {
	ReleaseSources  []ReleaseSourceConfig `yaml:"release_sources"`
	Slug            string                `yaml:"slug"`
	PreGaUserGroups []string              `yaml:"pre_ga_user_groups"`
	Releases        []ComponentSpec       `yaml:"releases"`
	TileNames       []string              `yaml:"tile_names"`
	Stemcell        Stemcell              `yaml:"stemcell_criteria"`
}

type ReleaseSourceConfig struct {
	Type            string `yaml:"type"`
	ID              string `yaml:"id"`
	Publishable     bool   `yaml:"publishable"`
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	AccessKeyId     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	PathTemplate    string `yaml:"path_template"`
	Endpoint        string `yaml:"endpoint"`
	Org             string `yaml:"org"`
	GithubToken     string `yaml:"github_token"`
}

// ComponentLock represents an exact build of a bosh release
// It may identify the where the release is cached;
// it may identify the stemcell used to compile the release.
//
// All fields must be comparable because this struct may be
// used as a key type in a map. Don't add array or map fields.
type ComponentLock struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version,omitempty"`
	SHA1    string `yaml:"sha1"`

	StemcellOS      string `yaml:"os,omitempty"`
	StemcellVersion string `yaml:"stemcell_version,omitempty"`

	RemoteSource string `yaml:"remote_source"`
	RemotePath   string `yaml:"remote_path"`
}

func (lock ComponentLock) Spec() ComponentSpec {
	return ComponentSpec{
		Name:            lock.Name,
		Version:         lock.Version,
		StemcellOS:      lock.StemcellOS,
		StemcellVersion: lock.StemcellVersion,
	}
}

func (lock ComponentLock) WithSHA1(sum string) ComponentLock {
	lock.SHA1 = sum
	return lock
}

func (lock ComponentLock) WithRemote(source, path string) ComponentLock {
	lock.RemoteSource = source
	lock.RemotePath = path
	return lock
}

func (lock ComponentLock) UnsetStemcell() ComponentLock {
	lock.StemcellOS = ""
	lock.StemcellVersion = ""
	return lock
}

func (k KilnfileLock) FindReleaseWithName(name string) (ComponentLock, error) {
	for _, r := range k.Releases {
		if r.Name == name {
			return r, nil
		}
	}
	return ComponentLock{}, errors.New("not found")
}
