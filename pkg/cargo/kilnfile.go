package cargo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
)

type Kilnfile struct {
	ReleaseSources  []ReleaseSourceConfig `yaml:"release_sources,omitempty"`
	Slug            string                `yaml:"slug,omitempty"`
	PreGaUserGroups []string              `yaml:"pre_ga_user_groups,omitempty"`
	Releases        []ComponentSpec       `yaml:"releases,omitempty"`
	TileNames       []string              `yaml:"tile_names,omitempty"`
	Stemcell        Stemcell              `yaml:"stemcell_criteria,omitempty"`
}

func (kf Kilnfile) ComponentSpec(name string) (ComponentSpec, error) {
	for _, s := range kf.Releases {
		if s.Name == name {
			return s, nil
		}
	}
	return ComponentSpec{}, fmt.Errorf("failed to find component specification with name %q in Kilnfile", name)
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
	return ComponentLock{}, errors.New("not found")
}

func (k KilnfileLock) UpdateReleaseLockWithName(name string, lock ComponentLock) error {
	for i, r := range k.Releases {
		if r.Name == name {
			k.Releases[i] = lock
			return nil
		}
	}
	return errors.New("not found")
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

	// GitHubRepository are where the BOSH release source code is
	GitHubRepository string `yaml:"github_repository,omitempty"`
}

func (spec ComponentSpec) VersionConstraints() (*semver.Constraints, error) {
	if spec.Version == "" {
		spec.Version = ">=0"
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

func (spec ComponentSpec) OSVersionSlug() boshdir.OSVersionSlug {
	return boshdir.NewOSVersionSlug(spec.StemcellOS, spec.StemcellVersion)
}

func (spec ComponentSpec) ReleaseSlug() boshdir.ReleaseSlug {
	return boshdir.NewReleaseSlug(spec.Name, spec.Version)
}

type ReleaseSourceConfig struct {
	Type            string `yaml:"type,omitempty"`
	ID              string `yaml:"id,omitempty"`
	Publishable     bool   `yaml:"publishable,omitempty"`
	Bucket          string `yaml:"bucket,omitempty"`
	Region          string `yaml:"region,omitempty"`
	AccessKeyId     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	PathTemplate    string `yaml:"path_template,omitempty"`
	Endpoint        string `yaml:"endpoint,omitempty"`
	Org             string `yaml:"org,omitempty"`
	GithubToken     string `yaml:"github_token,omitempty"`
	Repo            string `yaml:"repo,omitempty"`
	ArtifactoryHost string `yaml:"artifactory_host,omitempty"`
	Username        string `yaml:"username,omitempty"`
	Password        string `yaml:"password,omitempty"`
}

// ComponentLock represents an exact build of a bosh release
// It may identify the where the release is cached;
// it may identify the stemcell used to compile the release.
//
// All fields must be comparable because this struct may be
// used as a key type in a map. Don't add array or map fields.
type ComponentLock struct {
	Name    string `yaml:"name"`
	SHA1    string `yaml:"sha1"`
	Version string `yaml:"version,omitempty"`

	StemcellOS      string `yaml:"-"`
	StemcellVersion string `yaml:"-"`

	RemoteSource string `yaml:"remote_source"`
	RemotePath   string `yaml:"remote_path"`
}

func (lock ComponentLock) ReleaseSlug() boshdir.ReleaseSlug {
	return boshdir.NewReleaseSlug(lock.Name, lock.Version)
}

func (lock ComponentLock) StemcellSlug() boshdir.OSVersionSlug {
	return boshdir.NewOSVersionSlug(lock.StemcellOS, lock.StemcellVersion)
}

func (lock ComponentLock) String() string {
	var b strings.Builder
	b.WriteString(lock.Name)
	b.WriteByte(' ')
	b.WriteString(lock.Version)
	b.WriteByte(' ')

	if lock.SHA1 != "" {
		b.WriteString(lock.SHA1[:len(lock.SHA1)%8])
		b.WriteByte(' ')
	}

	if lock.StemcellOS != "" {
		b.WriteString(lock.StemcellOS)
		b.WriteByte(' ')
	}
	if lock.StemcellVersion != "" {
		b.WriteString(lock.StemcellVersion)
		b.WriteByte(' ')
	}

	if lock.RemoteSource != "" {
		b.WriteString(lock.RemoteSource)
		b.WriteByte(' ')
	}
	if lock.RemotePath != "" {
		b.WriteString(lock.RemotePath)
		b.WriteByte(' ')
	}

	return b.String()
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

func (lock ComponentLock) ParseVersion() (*semver.Version, error) {
	return semver.NewVersion(lock.Version)
}

type Stemcell struct {
	Alias   string `yaml:"alias,omitempty"`
	OS      string `yaml:"os"`
	Version string `yaml:"version"`
}
