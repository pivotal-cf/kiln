package component

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/google/go-github/v40/github"
	"golang.org/x/oauth2"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type GithubReleaseSource struct {
	cargo.ReleaseSourceConfig
	Token  string
	Logger *log.Logger
	Client *github.Client
}

// NewGithubReleaseSource will provision a new GithubReleaseSource Project
// from the Kilnfile (ReleaseSourceConfig). If type is incorrect it will PANIC
func NewGithubReleaseSource(c cargo.ReleaseSourceConfig) *GithubReleaseSource {

	if c.Type != "" && c.Type != ReleaseSourceTypeGithub {
		panic(panicMessageWrongReleaseSourceType)
	}
	if c.GithubToken == "" {
		panic("no token passed for github release source")
	}
	if c.Org == "" {
		panic("no github org passed for github release source")
	}

	ctx := context.TODO()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.GithubToken})
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(tokenClient)

	return &GithubReleaseSource{
		ReleaseSourceConfig: c,
		Logger:              log.New(os.Stdout, "[Github release source] ", log.Default().Flags()),
		Client:              githubClient,
	}
}

// Configuration returns the configuration of the ReleaseSource that came from the kilnfile.
// It should not be modified.
func (grs GithubReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return grs.ReleaseSourceConfig
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (grs GithubReleaseSource) GetMatchedRelease(s Spec) (Lock, error) {
	_, err := semver.NewVersion(s.Version)
	if err != nil {
		return Lock{}, fmt.Errorf("expected version to be an exact version")
	}
	return LockFromGithubRelease(context.TODO(), grs.Client.Repositories, grs.Org, s,
		GetGithubReleaseWithTag(grs.Client.Repositories, s.Version))
}

//counterfeiter:generate -o ./fakes/release_by_tag_getter.go --fake-name ReleaseByTagGetter . ReleaseByTagGetter

type ReleaseByTagGetter interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
}

func GetGithubReleaseWithTag(ghAPI ReleaseByTagGetter, tag string) GetGithubReleaseFunc {
	return func(ctx context.Context, repoOwner, repoName string) (*github.RepositoryRelease, error) {
		release, response, err := ghAPI.GetReleaseByTag(ctx, repoOwner, repoName, tag)
		if err != nil {
			return nil, err
		}
		return release, checkStatus(http.StatusOK, response.StatusCode)
	}
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (grs GithubReleaseSource) FindReleaseVersion(s Spec) (Lock, error) {
	c, err := s.VersionConstraints()
	if err != nil {
		return Lock{}, fmt.Errorf("expected version to be a constraint")
	}
	return LockFromGithubRelease(context.TODO(), grs.Client.Repositories, grs.Org, s,
		GetReleaseMatchingConstraint(grs.Client.Repositories, c))
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
func (grs GithubReleaseSource) DownloadRelease(releaseDir string, remoteRelease Lock) (Local, error) {
	grs.Logger.Printf(logLineDownload, remoteRelease.Name, ReleaseSourceTypeGithub, grs.ID)
	return downloadRelease(context.TODO(), releaseDir, remoteRelease, grs.Client, grs.Logger)
}

//counterfeiter:generate -o ./fakes_internal/github_new_request_doer.go --fake-name GithubNewRequestDoer . githubNewRequestDoer

type githubNewRequestDoer interface {
	NewRequest(method, urlStr string, body interface{}) (*http.Request, error)
	Do(ctx context.Context, req *http.Request, v interface{}) (*github.Response, error)
}

func downloadRelease(ctx context.Context, releaseDir string, remoteRelease Lock, client githubNewRequestDoer, logger *log.Logger) (Local, error) {
	filePath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", remoteRelease.Name, remoteRelease.Version))

	file, err := os.Create(filePath)
	if err != nil {
		return Local{}, err
	}
	defer func() { _ = file.Close() }()

	request, err := client.NewRequest(http.MethodGet, remoteRelease.RemotePath, nil)
	if err != nil {
		return Local{}, err
	}

	hash := sha1.New()
	response, err := client.Do(ctx, request, io.MultiWriter(file, hash))
	if err != nil {
		return Local{}, err
	}

	err = checkStatus(http.StatusOK, response.StatusCode)
	if err != nil {
		return Local{}, err
	}

	remoteRelease.SHA1 = hex.EncodeToString(hash.Sum(nil))

	return Local{Lock: remoteRelease, LocalPath: filePath}, nil
}

//counterfeiter:generate -o ./fakes/release_asset_downloader.go --fake-name ReleaseAssetDownloader . ReleaseAssetDownloader

type ReleaseAssetDownloader interface {
	DownloadReleaseAsset(ctx context.Context, owner, repo string, id int64, followRedirectsClient *http.Client) (rc io.ReadCloser, redirectURL string, err error)
}

type GetGithubReleaseFunc func(ctx context.Context, org, repo string) (*github.RepositoryRelease, error)

func LockFromGithubRelease(ctx context.Context, downloader ReleaseAssetDownloader, owner string, spec Spec, getRelease GetGithubReleaseFunc) (Lock, error) {
	for _, repoURL := range spec.GitRepositories {
		repoOwner, repoName, err := OwnerAndRepoFromGitHubURI(repoURL)
		if err != nil {
			continue
		}
		release, err := getRelease(ctx, repoOwner, repoName)
		if err != nil {
			return Lock{}, err
		}
		expectedAssetName := fmt.Sprintf("%s-%s.tgz", spec.Name, release.GetTagName())
		for _, asset := range release.Assets {
			if asset.GetName() != expectedAssetName {
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
				Version:      release.GetTagName(),
				RemoteSource: owner,
				RemotePath:   asset.GetBrowserDownloadURL(),
				SHA1:         sum,
			}, nil
		}
	}
	return Lock{}, ErrNotFound
}

func calculateSHA1(rc io.ReadCloser) (string, error) {
	defer func() {
		_ = rc.Close()
	}()
	w := sha1.New()
	_, err := io.Copy(w, rc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", w.Sum(nil)), nil
}
