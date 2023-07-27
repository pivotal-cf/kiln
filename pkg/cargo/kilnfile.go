package cargo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
)

type Kilnfile struct {
	ReleaseSources  []ReleaseSourceConfig             `yaml:"release_sources,omitempty"`
	Slug            string                            `yaml:"slug,omitempty"`
	PreGaUserGroups []string                          `yaml:"pre_ga_user_groups,omitempty"`
	Releases        []BOSHReleaseTarballSpecification `yaml:"releases,omitempty"`
	TileNames       []string                          `yaml:"tile_names,omitempty"`
	Stemcell        Stemcell                          `yaml:"stemcell_criteria,omitempty"`
}

func (kf Kilnfile) BOSHReleaseTarballSpecification(name string) (BOSHReleaseTarballSpecification, error) {
	for _, s := range kf.Releases {
		if s.Name == name {
			return s, nil
		}
	}
	return BOSHReleaseTarballSpecification{}, fmt.Errorf("failed to find component specification with name %q in Kilnfile", name)
}

type KilnfileLock struct {
	Releases []BOSHReleaseTarballLock `yaml:"releases"`
	Stemcell Stemcell                 `yaml:"stemcell_criteria"`
}

func (k KilnfileLock) FindBOSHReleaseWithName(name string) (BOSHReleaseTarballLock, error) {
	for _, r := range k.Releases {
		if r.Name == name {
			return r, nil
		}
	}
	return BOSHReleaseTarballLock{}, errors.New("not found")
}

func (k KilnfileLock) UpdateBOSHReleaseTarballLockWithName(name string, lock BOSHReleaseTarballLock) error {
	for i, r := range k.Releases {
		if r.Name == name {
			k.Releases[i] = lock
			return nil
		}
	}
	return errors.New("not found")
}

type BOSHReleaseTarballSpecification struct {
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

func (spec BOSHReleaseTarballSpecification) VersionConstraints() (*semver.Constraints, error) {
	if spec.Version == "" {
		spec.Version = ">=0"
	}
	c, err := semver.NewConstraint(spec.Version)
	if err != nil {
		return nil, fmt.Errorf("expected version to be a Constraint: %w", err)
	}
	return c, nil
}

func (spec BOSHReleaseTarballSpecification) Lock() BOSHReleaseTarballLock {
	return BOSHReleaseTarballLock{
		Name:            spec.Name,
		Version:         spec.Version,
		StemcellOS:      spec.StemcellOS,
		StemcellVersion: spec.StemcellVersion,
	}
}

func (spec BOSHReleaseTarballSpecification) OSVersionSlug() boshdir.OSVersionSlug {
	return boshdir.NewOSVersionSlug(spec.StemcellOS, spec.StemcellVersion)
}

func (spec BOSHReleaseTarballSpecification) ReleaseSlug() boshdir.ReleaseSlug {
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
	RoleARN         string `yaml:"role_arn,omitempty"`
	PathTemplate    string `yaml:"path_template,omitempty"`
	Endpoint        string `yaml:"endpoint,omitempty"`
	Org             string `yaml:"org,omitempty"`
	GithubToken     string `yaml:"github_token,omitempty"`
	Repo            string `yaml:"repo,omitempty"`
	ArtifactoryHost string `yaml:"artifactory_host,omitempty"`
	Username        string `yaml:"username,omitempty"`
	Password        string `yaml:"password,omitempty"`
}

// BOSHReleaseTarballLock represents an exact build of a bosh release
// It may identify the where the release is cached;
// it may identify the stemcell used to compile the release.
//
// All fields must be comparable because this struct may be
// used as a key type in a map. Don't add array or map fields.
type BOSHReleaseTarballLock struct {
	Name    string `yaml:"name" json:"name"`
	SHA1    string `yaml:"sha1" json:"sha1"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`

	StemcellOS      string `yaml:"-" json:"-"`
	StemcellVersion string `yaml:"-" json:"-"`

	RemoteSource string `yaml:"remote_source" json:"remote_source"`
	RemotePath   string `yaml:"remote_path" json:"remote_path"`
}

func (lock BOSHReleaseTarballLock) ReleaseSlug() boshdir.ReleaseSlug {
	return boshdir.NewReleaseSlug(lock.Name, lock.Version)
}

func (lock BOSHReleaseTarballLock) StemcellSlug() boshdir.OSVersionSlug {
	return boshdir.NewOSVersionSlug(lock.StemcellOS, lock.StemcellVersion)
}

func (lock BOSHReleaseTarballLock) String() string {
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

func (lock BOSHReleaseTarballLock) WithSHA1(sum string) BOSHReleaseTarballLock {
	lock.SHA1 = sum
	return lock
}

func (lock BOSHReleaseTarballLock) WithRemote(source, path string) BOSHReleaseTarballLock {
	lock.RemoteSource = source
	lock.RemotePath = path
	return lock
}

func (lock BOSHReleaseTarballLock) ParseVersion() (*semver.Version, error) {
	return semver.NewVersion(lock.Version)
}

type Stemcell struct {
	Alias        string `yaml:"alias,omitempty"`
	OS           string `yaml:"os"`
	Version      string `yaml:"version"`
	TanzuNetSlug string `yaml:"slug,omitempty"`
}

func (stemcell Stemcell) ProductSlug() (string, error) {
	if stemcell.TanzuNetSlug != "" {
		return stemcell.TanzuNetSlug, nil
	}
	// fall back behavior for compatability
	switch stemcell.OS {
	case "ubuntu-xenial":
		return "stemcells-ubuntu-xenial", nil
	case "ubuntu-jammy":
		return "stemcells-ubuntu-jammy", nil
	case "windows2019":
		return "stemcells-windows-server", nil
	default:
		return "", fmt.Errorf("stemcell slug not set for os %s", stemcell.OS)
	}
}
