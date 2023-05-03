package cargo

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
	"github.com/google/go-github/v40/github"
	"github.com/pivotal-cf/kiln/internal/gh"
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

func (kf Kilnfile) DownloadBOSHRelease(ctx context.Context, logger *log.Logger, lock ComponentLock, releasesDirectory string) (string, error) {
	source, found := releaseSourceByID(kf, lock.RemoteSource)
	if !found {
		return "", fmt.Errorf("bosh release source configuration not found in Kilnfile")
	}
	clients, err := configureDownloadClient(ctx, logger, source)
	if err != nil {
		return "", err
	}
	return downloadBOSHRelease(ctx, logger, source, lock, releasesDirectory, clients)
}

//func (kf Kilnfile) UpdateBOSHReleasesLock(ctx context.Context, logger *log.Logger, lock KilnfileLock, releasesDirectory string) (KilnfileLock, error) {
//	for i, componentLock := range lock.Releases {
//		updatedLock, err := kf.UpdateBOSHReleaseLock(ctx, logger, componentLock, releasesDirectory)
//		if err != nil {
//			return lock, err
//		}
//		lock.Releases[i] = updatedLock
//	}
//	return lock, nil
//}

//func (kf Kilnfile) UpdateBOSHReleaseLock(ctx context.Context, logger *log.Logger, lock ComponentLock, releasesDirectory string) (ComponentLock, error) {
//	panic("not implemented")
//}
//
//func (kf Kilnfile) UploadPublishableBOSHReleaseArtifactory(ctx context.Context, logger *log.Logger, lock KilnfileLock) (KilnfileLock, error) {
//	panic("not implemented")
//}

func releaseSourceByID(kilnfile Kilnfile, releaseSourceID string) (ReleaseSourceConfig, bool) {
	for _, config := range kilnfile.ReleaseSources {
		if ReleaseSourceID(config) == releaseSourceID {
			return config, true
		}
	}
	return ReleaseSourceConfig{}, false
}

type s3Downloader interface {
	DownloadWithContext(ctx aws.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

// clients wraps the client types used to download bosh release tarballs
type clients struct {
	artifactoryClient, boshIOClient *http.Client
	githubClient                    *github.Client
	s3Client                        s3iface.S3API
}

// configureDownloadClient sets the one download client field needed for releaseSourceConfiguration
func configureDownloadClient(ctx context.Context, _ *log.Logger, releaseSourceConfiguration ReleaseSourceConfig) (clients, error) {
	switch releaseSourceConfiguration.Type {
	case ReleaseSourceTypeArtifactory:
		return clients{
			artifactoryClient: http.DefaultClient,
		}, nil
	case ReleaseSourceTypeBOSHIO:
		return clients{
			boshIOClient: http.DefaultClient,
		}, nil
	case ReleaseSourceTypeGithub:
		client, err := githubAPIClient(ctx, releaseSourceConfiguration)
		return clients{
			githubClient: client,
		}, err
	case ReleaseSourceTypeS3:
		s3Client, err := awsS3Client(releaseSourceConfiguration)
		return clients{
			s3Client: s3Client,
		}, err
	default:
		return clients{}, fmt.Errorf("setup for BOSH release tarball source not implemented")
	}
}

func downloadBOSHRelease(ctx context.Context, logger *log.Logger, releaseSourceConfiguration ReleaseSourceConfig, lock ComponentLock, releasesDirectory string, clients clients) (string, error) {
	switch releaseSourceConfiguration.Type {
	case ReleaseSourceTypeArtifactory:
		downloadURL := releaseSourceConfiguration.ArtifactoryHost + "/" + path.Join("artifactory", releaseSourceConfiguration.Repo, lock.RemotePath)
		tarballFilePath := filepath.Join(releasesDirectory, filepath.Base(lock.RemotePath))
		return tarballFilePath, downloadTarballFromURL(ctx, clients.artifactoryClient, logger, lock, downloadURL, tarballFilePath)
	case ReleaseSourceTypeBOSHIO:
		tarballFilePath := filepath.Join(releasesDirectory, fmt.Sprintf("%s-%s.tgz", lock.Name, lock.Version))
		return tarballFilePath, downloadTarballFromURL(ctx, clients.boshIOClient, logger, lock, lock.RemotePath, tarballFilePath)
	case ReleaseSourceTypeGithub:
		tarballFilePath := filepath.Join(releasesDirectory, fmt.Sprintf("%s-%s.tgz", lock.Name, lock.Version))
		return downloadBOSHReleaseFromGitHub(ctx, logger, lock, clients.githubClient, tarballFilePath)
	case ReleaseSourceTypeS3:
		tarballFilePath := filepath.Join(releasesDirectory, filepath.Base(lock.RemotePath))
		file, err := os.Create(tarballFilePath)
		if err != nil {
			return "", err
		}
		defer closeAndIgnoreError(file)
		_, err = s3manager.NewDownloaderWithClient(clients.s3Client).DownloadWithContext(ctx, file, &s3.GetObjectInput{
			Bucket: aws.String(releaseSourceConfiguration.Bucket),
			Key:    aws.String(lock.RemotePath),
		})
		if err != nil {
			return "", err
		}
		return tarballFilePath, checkFilePathSum(file, lock)
	default:
		return "", fmt.Errorf("download for BOSH release tarball source not implemented")
	}
}

func downloadBOSHReleaseFromGitHub(ctx context.Context, logger *log.Logger, lock ComponentLock, githubClient *github.Client, tarballFilePath string) (string, error) {
	u, err := url.Parse(lock.RemotePath)
	if err != nil {
		return "", err
	}
	segments := strings.Split(u.Path, "/")
	if len(segments) < 2 {
		return "", fmt.Errorf("failed to parse repository name and owner from bosh release remote path")
	}
	repoOwner, repoName := segments[3], segments[4]
	rTag, _, err := githubClient.Repositories.GetReleaseByTag(ctx, repoOwner, repoName, lock.Version)
	if err != nil {
		logger.Println("warning: failed to find release tag of ", lock.Version)
		rTag, _, err = githubClient.Repositories.GetReleaseByTag(ctx, repoOwner, repoName, "v"+lock.Version)
		if err != nil {
			return "", fmt.Errorf("failed to find release tag: %w", err)
		}
	}
	assetFile, found := findGitHubReleaseAssetFile(rTag.Assets, lock)
	if !found {
		return "", errors.New("failed to download file for release: expected release asset not found")
	}
	rc, _, err := githubClient.Repositories.DownloadReleaseAsset(ctx, repoOwner, repoName, assetFile.GetID(), http.DefaultClient)
	if err != nil {
		fmt.Printf("failed to download file for release: %+v: ", err)
		return "", err
	}
	defer closeAndIgnoreError(rc)
	file, err := os.Create(tarballFilePath)
	if err != nil {
		fmt.Printf("failed to create file for release: %+v: ", err)
		return "", err
	}
	defer closeAndIgnoreError(file)
	return tarballFilePath, checkIOSum(file, rc, lock)
}

func awsS3Client(releaseSourceConfig ReleaseSourceConfig) (s3iface.S3API, error) {
	var config aws.Config
	if releaseSourceConfig.AccessKeyId != "" && !strings.HasPrefix(releaseSourceConfig.AccessKeyId, "$(") {
		config.Credentials = credentials.NewStaticCredentials(releaseSourceConfig.AccessKeyId, releaseSourceConfig.SecretAccessKey, "")
	}
	awsSession, err := session.NewSessionWithOptions(session.Options{
		Config: config,
	})
	if err != nil {
		return nil, err
	}
	return s3.New(awsSession), nil
}

// downloadTarballFromURL downloads a tarball from a URL that does not require authentication: bosh.io and Build Artifactory.
func downloadTarballFromURL(ctx context.Context, client *http.Client, _ *log.Logger, lock ComponentLock, downloadURL, filePath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		var dnsErr net.DNSError
		for errors.Is(err, &dnsErr) {
			return fmt.Errorf("failed to do HTTP request (are you connected to the VPN?): %w", err)
		}
		return err
	}
	defer closeAndIgnoreError(res.Body)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status code %d %q", res.StatusCode, http.StatusText(res.StatusCode))
	}
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(file)
	return checkIOSum(file, res.Body, lock)
}

func checkIOSum(w io.Writer, r io.Reader, lock ComponentLock) error {
	hash := sha1.New()

	mw := io.MultiWriter(w, hash)
	_, err := io.Copy(mw, r)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum for downloaded file: %+v: ", err)
	}

	if hex.EncodeToString(hash.Sum(nil)) != lock.SHA1 {
		return fmt.Errorf("lock checksum does not match downloaded file")
	}

	return nil
}

func checkFilePathSum(file *os.File, lock ComponentLock) error {
	_, err := file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("error reseting file cursor: %w", err) // untested
	}
	hash := sha1.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return fmt.Errorf("error hashing file contents: %w", err) // untested
	}
	if hex.EncodeToString(hash.Sum(nil)) != lock.SHA1 {
		return fmt.Errorf("lock checksum does not match downloaded file")
	}
	return nil
}

func findGitHubReleaseAssetFile(list []*github.ReleaseAsset, lock ComponentLock) (*github.ReleaseAsset, bool) {
	lockVersion := strings.TrimPrefix(lock.Version, "v")
	expectedAssetName := fmt.Sprintf("%s-%s.tgz", lock.Name, lockVersion)
	malformedAssetName := fmt.Sprintf("%s-v%s.tgz", lock.Name, lockVersion)
	for _, val := range list {
		switch val.GetName() {
		case expectedAssetName, malformedAssetName:
			return val, true
		}
	}
	return nil, false
}

func githubAPIClient(ctx context.Context, config ReleaseSourceConfig) (*github.Client, error) {
	githubHTTPClient, err := gh.HTTPClient(ctx, config.GithubToken)
	return github.NewClient(githubHTTPClient), err
}
