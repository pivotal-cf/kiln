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

	"github.com/Masterminds/semver/v3"
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
	Releases        []BOSHReleaseSpec     `yaml:"releases,omitempty"`
	TileNames       []string              `yaml:"tile_names,omitempty"`
	Stemcell        Stemcell              `yaml:"stemcell_criteria,omitempty"`
}

func (kf Kilnfile) ComponentSpec(name string) (BOSHReleaseSpec, error) {
	for _, s := range kf.Releases {
		if s.Name == name {
			return s, nil
		}
	}
	return BOSHReleaseSpec{}, fmt.Errorf("failed to find component specification with name %q in Kilnfile", name)
}

type KilnfileLock struct {
	Releases []BOSHReleaseLock `yaml:"releases"`
	Stemcell Stemcell          `yaml:"stemcell_criteria"`
}

func (k KilnfileLock) FindReleaseWithName(name string) (BOSHReleaseLock, error) {
	for _, r := range k.Releases {
		if r.Name == name {
			return r, nil
		}
	}
	return BOSHReleaseLock{}, errors.New("not found")
}

func (k KilnfileLock) UpdateReleaseLockWithName(name string, lock BOSHReleaseLock) error {
	for i, r := range k.Releases {
		if r.Name == name {
			k.Releases[i] = lock
			return nil
		}
	}
	return errors.New("not found")
}

type BOSHReleaseSpec struct {
	// Name is a required field and must be set with the bosh release name
	Name string `yaml:"name"`

	// Version if not set, it will default to ">0".
	// See https://github.com/Masterminds/semver/v3 for syntax
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

func (spec BOSHReleaseSpec) VersionConstraints() (*semver.Constraints, error) {
	if spec.Version == "" {
		spec.Version = ">=0"
	}
	c, err := semver.NewConstraint(spec.Version)
	if err != nil {
		return nil, fmt.Errorf("expected version to be a Constraint: %w", err)
	}
	return c, nil
}

func (spec BOSHReleaseSpec) Lock() BOSHReleaseLock {
	return BOSHReleaseLock{
		Name:            spec.Name,
		Version:         spec.Version,
		StemcellOS:      spec.StemcellOS,
		StemcellVersion: spec.StemcellVersion,
	}
}

func (spec BOSHReleaseSpec) OSVersionSlug() boshdir.OSVersionSlug {
	return boshdir.NewOSVersionSlug(spec.StemcellOS, spec.StemcellVersion)
}

func (spec BOSHReleaseSpec) ReleaseSlug() boshdir.ReleaseSlug {
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

// BOSHReleaseLock represents an exact build of a bosh release
// It may identify the where the release is cached;
// it may identify the stemcell used to compile the release.
//
// All fields must be comparable because this struct may be
// used as a key type in a map. Don't add array or map fields.
type BOSHReleaseLock struct {
	Name    string `yaml:"name"`
	SHA1    string `yaml:"sha1"`
	Version string `yaml:"version,omitempty"`

	StemcellOS      string `yaml:"-"`
	StemcellVersion string `yaml:"-"`

	RemoteSource string `yaml:"remote_source"`
	RemotePath   string `yaml:"remote_path"`
}

func (lock BOSHReleaseLock) ReleaseSlug() boshdir.ReleaseSlug {
	return boshdir.NewReleaseSlug(lock.Name, lock.Version)
}

func (lock BOSHReleaseLock) StemcellSlug() boshdir.OSVersionSlug {
	return boshdir.NewOSVersionSlug(lock.StemcellOS, lock.StemcellVersion)
}

func (lock BOSHReleaseLock) String() string {
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

func (lock BOSHReleaseLock) WithSHA1(sum string) BOSHReleaseLock {
	lock.SHA1 = sum
	return lock
}

func (lock BOSHReleaseLock) WithRemote(source, path string) BOSHReleaseLock {
	lock.RemoteSource = source
	lock.RemotePath = path
	return lock
}

func (lock BOSHReleaseLock) ParseVersion() (*semver.Version, error) {
	return semver.NewVersion(lock.Version)
}

type Stemcell struct {
	Alias   string `yaml:"alias,omitempty"`
	OS      string `yaml:"os"`
	Version string `yaml:"version"`
}

func (kf Kilnfile) DownloadBOSHReleaseTarball(ctx context.Context, logger *log.Logger, lock BOSHReleaseLock, releasesDirectory string) (string, error) {
	sourceConfig, found := releaseSourceByID(kf, lock.RemoteSource)
	if !found {
		return "", fmt.Errorf("bosh release source configuration not found in Kilnfile")
	}
	downloadClients, err := configureDownloadClient(ctx, logger, sourceConfig)
	if err != nil {
		return "", err
	}
	tarballPath, err := downloadBOSHRelease(ctx, logger, sourceConfig, lock, releasesDirectory, downloadClients)
	return tarballPath, err
}

func (kf Kilnfile) ListBOSHReleaseLocks(ctx context.Context, logger *log.Logger, boshReleaseName string) ([]BOSHReleaseLock, error) {
	return kf.ListBOSHReleaseLocksWithLimit(ctx, logger, boshReleaseName, 1)
}

func (kf Kilnfile) ListBOSHReleaseLocksWithLimit(ctx context.Context, logger *log.Logger, boshReleaseName string, limit int) ([]BOSHReleaseLock, error) {
	panic("not implemented")
}

func releaseSourceByID(kilnfile Kilnfile, releaseSourceID string) (ReleaseSourceConfig, bool) {
	for _, config := range kilnfile.ReleaseSources {
		if ReleaseSourceID(config) == releaseSourceID {
			return config, true
		}
	}
	return ReleaseSourceConfig{}, false
}

// clients wraps the client types used to download bosh release tarballs
type clients struct {
	artifactoryClient, boshIOClient *http.Client
	githubClient                    *github.Client
	s3Client                        s3iface.S3API
}

// configureDownloadClient sets the ONE download client field needed for releaseSourceConfiguration it does not bother
// with other release sources.
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

func downloadBOSHRelease(ctx context.Context, logger *log.Logger, releaseSourceConfiguration ReleaseSourceConfig, lock BOSHReleaseLock, releasesDirectory string, clients clients) (string, error) {
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
		return downloadBOSHReleaseFromS3(ctx, clients.s3Client, releaseSourceConfiguration, lock, tarballFilePath)
	default:
		return "", fmt.Errorf("download for BOSH release tarball source not implemented")
	}
}

func downloadBOSHReleaseFromS3(ctx context.Context, s3Client s3iface.S3API, releaseSourceConfiguration ReleaseSourceConfig, lock BOSHReleaseLock, tarballFilePath string) (string, error) {
	file, err := os.Create(tarballFilePath)
	if err != nil {
		return "", err
	}
	defer closeAndIgnoreError(file)
	_, err = s3manager.NewDownloaderWithClient(s3Client).DownloadWithContext(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(releaseSourceConfiguration.Bucket),
		Key:    aws.String(lock.RemotePath),
	})
	if err != nil {
		return "", err
	}
	return tarballFilePath, checkFilePathSum(file, lock)
}

func downloadBOSHReleaseFromGitHub(ctx context.Context, logger *log.Logger, lock BOSHReleaseLock, githubClient *github.Client, tarballFilePath string) (string, error) {
	u, err := url.Parse(lock.RemotePath)
	if err != nil {
		return "", err
	}
	segments := strings.Split(u.Path, "/")
	if len(segments) < 2 {
		return "", fmt.Errorf("failed to parse repository name and owner from bosh release remote path")
	}
	repoOwner, repoName := segments[1], segments[2]
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
	awsSession, err := session.NewSessionWithOptions(session.Options{
		Config: newAWSConfig(releaseSourceConfig),
	})
	if err != nil {
		return nil, err
	}
	return s3.New(awsSession), nil
}

func newAWSConfig(releaseSourceConfig ReleaseSourceConfig) aws.Config {
	var config aws.Config
	if releaseSourceConfig.AccessKeyId != "" && !strings.HasPrefix(releaseSourceConfig.AccessKeyId, "$(") {
		config.Credentials = credentials.NewStaticCredentials(releaseSourceConfig.AccessKeyId, releaseSourceConfig.SecretAccessKey, "")
	}
	if releaseSourceConfig.Region != "" {
		config.Region = aws.String(releaseSourceConfig.Region)
	}
	return config
}

// downloadTarballFromURL downloads a tarball from a URL that does not require authentication: bosh.io and Build Artifactory.
func downloadTarballFromURL(ctx context.Context, client *http.Client, _ *log.Logger, lock BOSHReleaseLock, downloadURL, filePath string) error {
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

func checkIOSum(w io.Writer, r io.Reader, lock BOSHReleaseLock) error {
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

func checkFilePathSum(file *os.File, lock BOSHReleaseLock) error {
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

func findGitHubReleaseAssetFile(list []*github.ReleaseAsset, lock BOSHReleaseLock) (*github.ReleaseAsset, bool) {
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
	if err != nil {
		log.Println("CONFIGURATION WARNING", err)
		return github.NewClient(http.DefaultClient), nil
	}
	return github.NewClient(githubHTTPClient), err
}
