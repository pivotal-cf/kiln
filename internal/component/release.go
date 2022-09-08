package component

// TODO: move to cargo

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
)

type Spec struct {
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

func (spec Spec) VersionConstraints() (*semver.Constraints, error) {
	if spec.Version == "" {
		spec.Version = ">0"
	}
	c, err := semver.NewConstraint(spec.Version)
	if err != nil {
		return nil, fmt.Errorf("expected version to be a Constraint: %w", err)
	}
	return c, nil
}

func (spec Spec) Lock() Lock {
	return Lock{
		Name:            spec.Name,
		Version:         spec.Version,
		StemcellOS:      spec.StemcellOS,
		StemcellVersion: spec.StemcellVersion,
	}
}

func (spec Spec) OSVersionSlug() boshdir.OSVersionSlug {
	return boshdir.NewOSVersionSlug(spec.StemcellOS, spec.StemcellVersion)
}

func (spec Spec) ReleaseSlug() boshdir.ReleaseSlug {
	return boshdir.NewReleaseSlug(spec.Name, spec.Version)
}

func (spec Spec) UnsetStemcell() Spec {
	spec.StemcellOS = ""
	spec.StemcellVersion = ""
	return spec
}

// Lock represents an exact build of a bosh release
// It may identify the where the release is cached;
// it may identify the stemcell used to compile the release.
//
// All fields must be comparable because this struct may be
// used as a key type in a map. Don't add array or map fields.
type Lock struct {
	Name    string `yaml:"name"`
	SHA1    string `yaml:"sha1"`
	Version string `yaml:"version,omitempty"`

	StemcellOS      string `yaml:"-"`
	StemcellVersion string `yaml:"-"`

	RemoteSource string `yaml:"remote_source"`
	RemotePath   string `yaml:"remote_path"`
}

func (lock Lock) ReleaseSlug() boshdir.ReleaseSlug {
	return boshdir.NewReleaseSlug(lock.Name, lock.Version)
}

func (lock Lock) StemcellSlug() boshdir.OSVersionSlug {
	return boshdir.NewOSVersionSlug(lock.StemcellOS, lock.StemcellVersion)
}

func (lock Lock) String() string {
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

func (lock Lock) Spec() Spec {
	return Spec{
		Name:            lock.Name,
		Version:         lock.Version,
		StemcellOS:      lock.StemcellOS,
		StemcellVersion: lock.StemcellVersion,
	}
}

func (lock Lock) WithSHA1(sum string) Lock {
	lock.SHA1 = sum
	return lock
}

func (lock Lock) WithRemote(source, path string) Lock {
	lock.RemoteSource = source
	lock.RemotePath = path
	return lock
}

func (lock Lock) UnsetStemcell() Lock {
	lock.StemcellOS = ""
	lock.StemcellVersion = ""
	return lock
}

func (lock Lock) ParseVersion() (*semver.Version, error) {
	return semver.NewVersion(lock.Version)
}

type Local struct {
	Lock
	LocalPath string
}
