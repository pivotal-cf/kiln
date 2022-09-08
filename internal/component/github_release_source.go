package component

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/internal/gh"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/v40/github"
)

type GitHubReleaseSource struct {
	Identifier  string `yaml:"id,omitempty"`
	Publishable bool   `yaml:"publishable,omitempty"`

	Org         string `yaml:"org,omitempty"`
	GithubToken string `yaml:"github_token,omitempty"`

	collaborators struct {
		initOnce sync.Once
		client   *github.Client
	}
}

func (src *GitHubReleaseSource) ConfigurationErrors() []error {
	var result []error
	if src.GithubToken == "" {
		result = append(result, fmt.Errorf("no token passed for github release source"))
	}
	if src.Org == "" {
		result = append(result, fmt.Errorf("no github org passed for github release source"))
	}
	return result
}

func (src *GitHubReleaseSource) ID() string {
	if src.Identifier != "" {
		return src.Identifier
	}
	return src.Org
}

func (src *GitHubReleaseSource) IsPublishable() bool { return src.Publishable }
func (src *GitHubReleaseSource) Type() string        { return ReleaseSourceTypeGithub }

func (src *GitHubReleaseSource) init(ctx context.Context) error {
	src.collaborators.initOnce.Do(func() {
		src.collaborators.client = gh.Client(ctx, src.GithubToken)
	})
	return nil
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (src *GitHubReleaseSource) GetMatchedRelease(ctx context.Context, logger *log.Logger, spec Spec) (Lock, error) {
	if err := src.init(ctx); err != nil {
		return Lock{}, err
	}
	_, err := semver.NewVersion(spec.Version)
	if err != nil {
		return Lock{}, fmt.Errorf("expected version to be an exact version")
	}
	return LockFromGithubRelease(ctx, src.collaborators.client.Repositories, src.Org, spec,
		GetGithubReleaseWithTag(src.collaborators.client.Repositories, spec.Version))
}

//counterfeiter:generate -o ./fakes/release_by_tag_getter.go --fake-name ReleaseByTagGetter . ReleaseByTagGetter

type ReleaseByTagGetter interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
}

func GetGithubReleaseWithTag(ghAPI ReleaseByTagGetter, tag string) GetGithubReleaseFunc {
	return func(ctx context.Context, repoOwner, repoName string) (*github.RepositoryRelease, error) {
		release, response, err := ghAPI.GetReleaseByTag(ctx, repoOwner, repoName, "v"+tag)
		if err == nil {
			err = checkStatus(http.StatusOK, response.StatusCode)
		}
		if err != nil {
			release, response, err = ghAPI.GetReleaseByTag(ctx, repoOwner, repoName, tag)
			if err == nil {
				err = checkStatus(http.StatusOK, response.StatusCode)
			}
			if err != nil {
				return nil, err
			}
		}
		return release, nil
	}
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (src *GitHubReleaseSource) FindReleaseVersion(ctx context.Context, logger *log.Logger, spec Spec) (Lock, error) {
	if err := src.init(ctx); err != nil {
		return Lock{}, err
	}
	c, err := spec.VersionConstraints()
	if err != nil {
		return Lock{}, fmt.Errorf("expected version to be a constraint")
	}
	return LockFromGithubRelease(ctx, src.collaborators.client.Repositories, src.Org, spec,
		GetReleaseMatchingConstraint(src.collaborators.client.Repositories, c))
}

//counterfeiter:generate -o ./fakes/releases_lister.go --fake-name ReleasesLister . ReleasesLister

type ReleasesLister interface {
	ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
}

func GetReleaseMatchingConstraint(ghAPI ReleasesLister, constraints *semver.Constraints) GetGithubReleaseFunc {
	return func(ctx context.Context, repoOwner, repoName string) (*github.RepositoryRelease, error) {
		ops := &github.ListOptions{}

		var (
			highestMatchingVersion               *semver.Version
			matchingReleases                     *github.RepositoryRelease
			numberOfPagesWithoutMatchingVersions = 0
		)
		for numberOfPagesWithoutMatchingVersions < 2 {
			releases, response, err := ghAPI.ListReleases(ctx, repoOwner, repoName, ops)
			if err != nil {
				return nil, err
			}
			err = checkStatus(http.StatusOK, response.StatusCode)
			if err != nil {
				break
			}

			foundHigherVersion := false
			for _, release := range releases {
				v, err := semver.NewVersion(release.GetTagName())
				if err != nil {
					continue
				}
				if !constraints.Check(v) {
					continue
				}
				if highestMatchingVersion != nil && v.LessThan(highestMatchingVersion) {
					continue
				}
				matchingReleases = release
				highestMatchingVersion = v
				foundHigherVersion = true
			}
			if foundHigherVersion {
				numberOfPagesWithoutMatchingVersions = 0
			} else {
				numberOfPagesWithoutMatchingVersions++
			}

			ops.Page++
		}

		if matchingReleases != nil {
			return matchingReleases, nil
		}

		return nil, ErrNotFound
	}
}

// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
// It should also calculate and set the SHA1 field on the Local result; it does not need
// to ensure the sums match, the caller must verify this.
func (src *GitHubReleaseSource) DownloadRelease(ctx context.Context, logger *log.Logger, releasesDir string, remoteRelease Lock) (Local, error) {
	if err := src.init(ctx); err != nil {
		return Local{}, err
	}
	logger.Printf(logLineDownload, remoteRelease.Name, ReleaseSourceTypeGithub, src.ID())
	return downloadRelease(ctx, releasesDir, remoteRelease, src.collaborators.client.Repositories, logger)
}

//counterfeiter:generate -o ./fakes/release_by_tag_getter_asset_downloader.go --fake-name ReleaseByTagGetterAssetDownloader . releaseByTagGetterAssetDownloader

type releaseByTagGetterAssetDownloader interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
	DownloadReleaseAsset(ctx context.Context, owner, repo string, id int64, followRedirectsClient *http.Client) (rc io.ReadCloser, redirectURL string, err error)
}

func downloadRelease(ctx context.Context, releaseDir string, remoteRelease Lock, client releaseByTagGetterAssetDownloader, _ *log.Logger) (Local, error) {
	filePath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", remoteRelease.Name, remoteRelease.Version))

	remoteUrl, err := url.Parse(remoteRelease.RemotePath)
	if err != nil {
		return Local{}, fmt.Errorf("failed to parse remote_path as url: %w", err)
	}
	remotePathParts := strings.Split(remoteUrl.Path, "/")
	// TODO: add test coverage for length
	org, repo := remotePathParts[1], remotePathParts[2]

	rTag, _, err := client.GetReleaseByTag(ctx, org, repo, remoteRelease.Version)
	if err != nil {
		log.Println("warning: failed to find release tag of ", remoteRelease.Version)
		rTag, _, err = client.GetReleaseByTag(ctx, org, repo, "v"+remoteRelease.Version)
		if err != nil {
			return Local{}, fmt.Errorf("cant find release tag: %+v\n", err.Error())
		}
	}

	assetFile, found := findAssetFile(rTag.Assets, remoteRelease)
	if !found {
		return Local{}, errors.New("failed to download file for release: expected release asset not found")
	}

	rc, _, err := client.DownloadReleaseAsset(ctx, org, repo, assetFile.GetID(), http.DefaultClient)
	if err != nil {
		fmt.Printf("failed to download file for release: %+v: ", err)
		return Local{}, err
	}
	defer closeAndIgnoreError(rc)

	file, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("failed to create file for release: %+v: ", err)
		return Local{}, err
	}
	defer closeAndIgnoreError(file)

	hash := sha1.New()

	mw := io.MultiWriter(file, hash)
	_, err = io.Copy(mw, rc)
	if err != nil {
		return Local{}, fmt.Errorf("failed to calculate checksum for downloaded file: %+v: ", err)
	}

	remoteRelease.SHA1 = hex.EncodeToString(hash.Sum(nil))

	return Local{Lock: remoteRelease, LocalPath: filePath}, nil
}

type ReleaseAssetDownloader interface {
	DownloadReleaseAsset(ctx context.Context, owner, repo string, id int64, followRedirectsClient *http.Client) (rc io.ReadCloser, redirectURL string, err error)
}

type GetGithubReleaseFunc func(ctx context.Context, org, repo string) (*github.RepositoryRelease, error)

func LockFromGithubRelease(ctx context.Context, downloader ReleaseAssetDownloader, owner string, spec Spec, getRelease GetGithubReleaseFunc) (Lock, error) {
	if spec.GitHubRepository == "" {
		return Lock{}, ErrNotFound
	}

	repoOwner, repoName, err := gh.OwnerAndRepoFromURI(spec.GitHubRepository)
	if err != nil {
		return Lock{}, ErrNotFound
	}

	if repoOwner != owner {
		return Lock{}, ErrNotFound
	}

	release, err := getRelease(ctx, repoOwner, repoName)
	if err != nil {
		return Lock{}, err
	}

	lockVersion := strings.TrimPrefix(release.GetTagName(), "v")
	expectedAssetName := fmt.Sprintf("%s-%s.tgz", spec.Name, lockVersion)
	malformedAssetName := fmt.Sprintf("%s-v%s.tgz", spec.Name, lockVersion)
	for _, asset := range release.Assets {
		switch asset.GetName() {
		case expectedAssetName, malformedAssetName:
		default:
			continue
		}

		rc, _, err := downloader.DownloadReleaseAsset(ctx, repoOwner, repoName, *asset.ID, http.DefaultClient)
		if err != nil {
			return Lock{}, err
		}
		sum, err := calculateSHA1(rc)
		if err != nil {
			return Lock{}, err
		}
		return Lock{
			Name:         spec.Name,
			Version:      lockVersion,
			RemoteSource: owner,
			RemotePath:   asset.GetBrowserDownloadURL(),
			SHA1:         sum,
		}, nil
	}

	return Lock{}, fmt.Errorf("no matching GitHub release asset file name equal to %q", expectedAssetName)
}

func findAssetFile(list []*github.ReleaseAsset, lock Lock) (*github.ReleaseAsset, bool) {
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

func calculateSHA1(rc io.ReadCloser) (string, error) {
	defer closeAndIgnoreError(rc)
	w := sha1.New()
	_, err := io.Copy(w, rc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", w.Sum(nil)), nil
}
