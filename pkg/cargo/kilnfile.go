package cargo

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Masterminds/semver/v3"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
	"github.com/crhntr/bijection"
)

const (
	unconstrainedVersion = "*"
)

type Kilnfile struct {
	ReleaseSources     []ReleaseSourceConfig             `yaml:"release_sources,omitempty"`
	Slug               string                            `yaml:"slug,omitempty"`
	PreGaUserGroups    []string                          `yaml:"pre_ga_user_groups,omitempty"`
	Releases           []BOSHReleaseTarballSpecification `yaml:"releases,omitempty"`
	TileNames          []string                          `yaml:"tile_names,omitempty"`
	Stemcell           Stemcell                          `yaml:"stemcell_criteria,omitempty"`
	BakeConfigurations []BakeConfiguration               `yaml:"bake_configurations"`
}

func (kf *Kilnfile) BOSHReleaseTarballSpecification(name string) (BOSHReleaseTarballSpecification, error) {
	for _, s := range kf.Releases {
		if s.Name == name {
			return s, nil
		}
	}
	return BOSHReleaseTarballSpecification{}, fmt.Errorf("failed to find component specification with name %q in Kilnfile", name)
}

func (kf *Kilnfile) Glaze(kl KilnfileLock) error {
	kf.Stemcell.Version = kl.Stemcell.Version
	for index, spec := range kf.Releases {
		if spec.FloatAlways {
			continue
		}
		lock, err := kl.FindBOSHReleaseWithName(spec.Name)
		if err != nil {
			return fmt.Errorf("release with name %q not found in Kilnfile.lock: %w", spec.Name, err)
		}
		kf.Releases[index].Version = lock.Version
	}
	return nil
}

func (kf *Kilnfile) DeGlaze(kl KilnfileLock) error {
	// TODO: what do about the stemcell???
	for index, spec := range kf.Releases {
		lock, err := kl.FindBOSHReleaseWithName(spec.Name)
		if err != nil {
			return fmt.Errorf("release with name %q not found in Kilnfile.lock: %w", spec.Name, err)
		}
		deGlazed, err := deGlazeBOSHReleaseTarballSpecification(spec, lock)
		if err != nil {
			return err
		}
		kf.Releases[index] = deGlazed
	}
	return nil
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

	// DeGlazeBehavior changes how version filed changes when de-glaze is run.
	DeGlazeBehavior DeGlazeBehavior `yaml:"maintenance_version_bump_policy"`

	// FloatAlways when does not override version constraint.
	// It skips locking it during Kilnfile.Glaze.
	FloatAlways bool `yaml:"float_always,omitempty"`

	// TeamSlackChannel slack channel for team that maintains this bosh release
	TeamSlackChannel string `yaml:"slack,omitempty"`
}

func (spec BOSHReleaseTarballSpecification) VersionConstraints() (*semver.Constraints, error) {
	if spec.Version == "" {
		spec.Version = unconstrainedVersion
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
	Name    string `yaml:"name"`
	SHA1    string `yaml:"sha1"`
	Version string `yaml:"version,omitempty"`

	StemcellOS      string `yaml:"-"`
	StemcellVersion string `yaml:"-"`

	RemoteSource string `yaml:"remote_source"`
	RemotePath   string `yaml:"remote_path"`
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

type DeGlazeBehavior int

const (
	LockNone  DeGlazeBehavior = iota - 1
	LockMajor                 // default value
	LockMinor
	LockPatch
)

var deGlazeStrings = bijection.New(map[DeGlazeBehavior]string{
	LockNone:  "LockNone",
	LockPatch: "LockPatch",
	LockMinor: "LockMinor",
	LockMajor: "LockMajor",
})

//goland:noinspection GoMixedReceiverTypes
func (dgb DeGlazeBehavior) String() string {
	buf, _ := dgb.MarshalText()
	return string(buf)
}

//goland:noinspection GoMixedReceiverTypes
func (dgb DeGlazeBehavior) MarshalText() (text []byte, err error) {
	s, found := deGlazeStrings.GetB(dgb)
	if !found {
		return nil, fmt.Errorf("unknown behavior")
	}
	return []byte(s), nil
}

//goland:noinspection GoMixedReceiverTypes
func (dgb *DeGlazeBehavior) UnmarshalText(text []byte) error {
	behavior, found := deGlazeStrings.GetA(string(text))
	if !found {
		return fmt.Errorf("unknown behavior")
	}
	*dgb = behavior
	return nil
}

//goland:noinspection GoMixedReceiverTypes
func (dgb DeGlazeBehavior) MarshalYAML() (any, error) {
	s, found := deGlazeStrings.GetB(dgb)
	if !found {
		return nil, fmt.Errorf("unknown behavior")
	}
	return s, nil
}

//goland:noinspection GoMixedReceiverTypes
func (dgb *DeGlazeBehavior) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	behavior, found := deGlazeStrings.GetA(s)
	if !found {
		return fmt.Errorf("unknown behavior")
	}
	*dgb = behavior
	return nil
}

//goland:noinspection GoMixedReceiverTypes
func (dgb DeGlazeBehavior) createConstraint(lockVersion string) (string, error) {
	v, err := BOSHReleaseTarballLock{Version: lockVersion}.ParseVersion()
	if err != nil {
		return "", err
	}
	var versionConstraint string
	switch dgb {
	case LockNone:
		versionConstraint = unconstrainedVersion
	case LockPatch:
		versionConstraint = fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch())
	case LockMinor:
		versionConstraint = fmt.Sprintf("~%d.%d", v.Major(), v.Minor())
	case LockMajor:
		versionConstraint = fmt.Sprintf("~%d", v.Major())
	}
	return versionConstraint, nil
}

func deGlazeBOSHReleaseTarballSpecification(spec BOSHReleaseTarballSpecification, lock BOSHReleaseTarballLock) (BOSHReleaseTarballSpecification, error) {
	var err error
	spec.Version, err = spec.DeGlazeBehavior.createConstraint(lock.Version)
	return spec, err
}

type BakeConfiguration struct {
	TileName                 string   `yaml:"tile_name,omitempty"                          json:"tile_name,omitempty"`
	Metadata                 string   `yaml:"metadata_filepath,omitempty"                  json:"metadata_filepath,omitempty"`
	FormDirectories          []string `yaml:"forms_directories,omitempty"                  json:"forms_directories,omitempty"`
	IconPath                 string   `yaml:"icon_filepath,omitempty"                      json:"icon_filepath,omitempty"`
	InstanceGroupDirectories []string `yaml:"instance_groups_directories,omitempty"        json:"instance_groups_directories,omitempty"`
	JobDirectories           []string `yaml:"jobs_directories,omitempty"                   json:"jobs_directories,omitempty"`
	MigrationDirectories     []string `yaml:"migrations_directories,omitempty"             json:"migrations_directories,omitempty"`
	PropertyDirectories      []string `yaml:"properties_directories,omitempty"             json:"properties_directories,omitempty"`
	RuntimeConfigDirectories []string `yaml:"runtime_configurations_directories,omitempty" json:"runtime_configurations_directories,omitempty"`
	BOSHVariableDirectories  []string `yaml:"bosh_variables_directories,omitempty"         json:"bosh_variables_directories,omitempty"`
	EmbedPaths               []string `yaml:"embed_paths,omitempty"                        json:"embed_paths,omitempty"`
	VariableFiles            []string `yaml:"variable_files,omitempty"                     json:"variable_files,omitempty"`
}

func KilnfileTemplate(name string) (Kilnfile, error) {
	const (
		artifactory = BOSHReleaseTarballSourceTypeArtifactory
		boshIO      = BOSHReleaseTarballSourceTypeBOSHIO
	)
	switch name {
	default:
		return Kilnfile{}, fmt.Errorf("unknown Kilnfile template please use one of %s", strings.Join([]string{
			artifactory, boshIO,
		}, ", "))
	case artifactory:
		return Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{
					Type:            BOSHReleaseTarballSourceTypeArtifactory,
					ID:              "compiled-releases",
					Publishable:     true,
					ArtifactoryHost: `{{ variable "artifactory_host" }}`,
					Repo:            `{{ variable "artifactory_repository" }}`,
					Username:        `{{ variable "artifactory_username" }}`,
					Password:        `{{ variable "artifactory_password" }}`,
					PathTemplate:    `compiled-releases/{{.Name}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
				},
				{
					Type:            BOSHReleaseTarballSourceTypeArtifactory,
					ID:              "ingested-releases",
					Publishable:     true,
					ArtifactoryHost: `{{ variable "artifactory_host" }}`,
					Repo:            `{{ variable "artifactory_repository" }}`,
					Username:        `{{ variable "artifactory_username" }}`,
					Password:        `{{ variable "artifactory_password" }}`,
					PathTemplate:    `bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz`,
				},
			},
		}, nil
	case boshIO:
		return Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{Type: BOSHReleaseTarballSourceTypeBOSHIO},
			},
		}, nil
	}
}
