package component

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v50/github"

	"github.com/pivotal-cf/kiln/internal/gh"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type GithubReleaseSource struct {
	cargo.ReleaseSourceConfig
	Token  string
	Logger *log.Logger

	ReleaseAssetDownloader
	ReleasesLister
	ReleaseByTagGetter
}

// NewGithubReleaseSource will provision a new GithubReleaseSource Project
// from the Kilnfile (ReleaseSourceConfig). If type is incorrect it will PANIC
func NewGithubReleaseSource(c cargo.ReleaseSourceConfig, logger *log.Logger) *GithubReleaseSource {
	if c.Type != "" && c.Type != ReleaseSourceTypeGithub {
		panic(panicMessageWrongReleaseSourceType)
	}

	if c.GithubToken == "" { // TODO remove this
		panic("no token passed for github release source")
	}

	if c.Org == "" {
		panic("no github org passed for github release source")
	}

	// The GitClient should be initialized with the proper host according to
	// the release repository URL instead. This function doesn't have access
	// to each release, so this will do for now.
	//
	host := ""
	if c.Org == "TNZ" {
		host = "https://github.gwd.broadcom.net"
	}

	if logger == nil {
		logger = log.New(os.Stderr, "[Github release source] ", log.Default().Flags())
	}

	githubClient, err := gh.GitClient(context.TODO(), host, c.GithubToken, c.GithubToken)
	if err != nil {
		panic(err)
	}
	return &GithubReleaseSource{
		ReleaseSourceConfig: c,
		Token:               c.GithubToken,
		Logger:              logger,

		ReleaseAssetDownloader: githubClient.Repositories,
		ReleaseByTagGetter:     githubClient.Repositories,
		ReleasesLister:         githubClient.Repositories,
	}
}

// Configuration returns the configuration of the ReleaseSource that came from the kilnfile.
// It should not be modified.
func (grs *GithubReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return grs.ReleaseSourceConfig
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (grs *GithubReleaseSource) GetMatchedRelease(s cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
	_, err := semver.NewVersion(s.Version)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, fmt.Errorf("expected version to be an exact version")
	}

	ctx := context.TODO()
	release, err := grs.GetGithubReleaseWithTag(ctx, s)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	return grs.getLockFromRelease(ctx, release, s, false)
}

//counterfeiter:generate -o ./fakes/release_by_tag_getter.go --fake-name ReleaseByTagGetter . ReleaseByTagGetter

type ReleaseByTagGetter interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
}

func (grs *GithubReleaseSource) GetGithubReleaseWithTag(ctx context.Context, s cargo.BOSHReleaseTarballSpecification) (*github.RepositoryRelease, error) {
	repoOwner, repoName, err := gh.RepositoryOwnerAndNameFromPath(s.GitHubRepository)
	if err != nil {
		return nil, ErrNotFound
	}

	if repoOwner != grs.Org {
		grs.Logger.Printf("GitHubRepository owner %q does not match configured Org %q, skipping...", repoOwner, grs.Org)
		return nil, ErrNotFound
	}

	release, response, err := grs.GetReleaseByTag(ctx, repoOwner, repoName, "v"+s.Version)
	if err == nil {
		err = checkStatus(http.StatusOK, response.StatusCode)
	}
	if err != nil {
		release, response, err = grs.GetReleaseByTag(ctx, repoOwner, repoName, s.Version)
		if err == nil {
			err = checkStatus(http.StatusOK, response.StatusCode)
		}
		if err != nil {
			return nil, err
		}
	}
	return release, nil
}

func (grs *GithubReleaseSource) GetLatestMatchingRelease(ctx context.Context, s cargo.BOSHReleaseTarballSpecification) (*github.RepositoryRelease, error) {
	c, err := s.VersionConstraints()
	if err != nil {
		return nil, fmt.Errorf("expected version to be a constraint")
	}

	repoOwner, repoName, err := gh.RepositoryOwnerAndNameFromPath(s.GitHubRepository)
	if err != nil {
		return nil, ErrNotFound
	}

	if repoOwner != grs.Org {
		return nil, ErrNotFound
	}

	ops := &github.ListOptions{}

	var (
		highestMatchingVersion               *semver.Version
		matchingReleases                     *github.RepositoryRelease
		numberOfPagesWithoutMatchingVersions = 0
	)
	for numberOfPagesWithoutMatchingVersions < 2 {
		releases, response, err := grs.ListReleases(ctx, repoOwner, repoName, ops)
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
			if !c.Check(v) {
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

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (grs *GithubReleaseSource) FindReleaseVersion(s cargo.BOSHReleaseTarballSpecification, noDownload bool) (cargo.BOSHReleaseTarballLock, error) {
	ctx := context.TODO()
	release, err := grs.GetLatestMatchingRelease(ctx, s)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	return grs.getLockFromRelease(ctx, release, s, noDownload)
}

func (grs *GithubReleaseSource) getLockFromRelease(ctx context.Context, r *github.RepositoryRelease, s cargo.BOSHReleaseTarballSpecification, noDownload bool) (cargo.BOSHReleaseTarballLock, error) {
	lockVersion := strings.TrimPrefix(r.GetTagName(), "v")
	expectedAssetName := fmt.Sprintf("%s-%s.tgz", s.Name, lockVersion)
	malformedAssetName := fmt.Sprintf("%s-v%s.tgz", s.Name, lockVersion)

	for _, asset := range r.Assets {
		switch asset.GetName() {
		case expectedAssetName, malformedAssetName:
		default:
			continue
		}

		sum := "not-calculated"
		if !noDownload {
			var err error
			sum, err = grs.getReleaseSHA1(ctx, s, *asset.ID)
			if err != nil {
				return cargo.BOSHReleaseTarballLock{}, err
			}
		}

		return cargo.BOSHReleaseTarballLock{
			Name:         s.Name,
			Version:      lockVersion,
			RemoteSource: grs.Org,
			RemotePath:   asset.GetBrowserDownloadURL(),
			SHA1:         sum,
		}, nil
	}

	return cargo.BOSHReleaseTarballLock{}, errors.Join(ErrNotFound, fmt.Errorf("no matching GitHub release asset file name equal to %q", expectedAssetName))
}

func (grs *GithubReleaseSource) getReleaseSHA1(ctx context.Context, s cargo.BOSHReleaseTarballSpecification, id int64) (string, error) {
	repoOwner, repoName, err := gh.RepositoryOwnerAndNameFromPath(s.GitHubRepository)
	if err != nil {
		return "", fmt.Errorf("could not parse repository name: %w", err)
	}

	rc, _, err := grs.DownloadReleaseAsset(ctx, repoOwner, repoName, id, http.DefaultClient)
	if err != nil {
		return "", err
	}
	return calculateSHA1(rc)
}

//counterfeiter:generate -o ./fakes/releases_lister.go --fake-name ReleasesLister . ReleasesLister

type ReleasesLister interface {
	ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
}

// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
// It should also calculate and set the SHA1 field on the Local result; it does not need
// to ensure the sums match, the caller must verify this.
func (grs *GithubReleaseSource) DownloadRelease(releaseDir string, remoteRelease cargo.BOSHReleaseTarballLock) (Local, error) {
	grs.Logger.Printf(logLineDownload, remoteRelease.Name, remoteRelease.Version, ReleaseSourceTypeGithub, grs.ID)
	return downloadRelease(context.TODO(), releaseDir, remoteRelease, grs, grs.Logger)
}

//counterfeiter:generate -o ./fakes/release_by_tag_getter_asset_downloader.go --fake-name ReleaseByTagGetterAssetDownloader . ReleaseByTagGetterAssetDownloader

type ReleaseByTagGetterAssetDownloader interface {
	ReleaseByTagGetter
	ReleaseAssetDownloader
}

func downloadRelease(ctx context.Context, releaseDir string, remoteRelease cargo.BOSHReleaseTarballLock, client ReleaseByTagGetterAssetDownloader, logger *log.Logger) (Local, error) {
	filePath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", remoteRelease.Name, remoteRelease.Version))

	remoteUrl, err := url.Parse(remoteRelease.RemotePath)
	if err != nil {
		return Local{}, fmt.Errorf("failed to parse remote_path as url: %w", err)
	}
	remotePathParts := strings.Split(remoteUrl.Path, "/")
	// TODO: add test coverage for length
	org, repo := remotePathParts[1], remotePathParts[2]

	rTag, _, err := client.GetReleaseByTag(ctx, org, repo, "v"+remoteRelease.Version)
	if err != nil {
		logger.Println("warning: failed to find release tag of", "v"+remoteRelease.Version)
		rTag, _, err = client.GetReleaseByTag(ctx, org, repo, remoteRelease.Version)
		if err != nil {
			return Local{}, fmt.Errorf("cant find release tag: %w", err)
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
		return Local{}, fmt.Errorf("failed to calculate checksum for downloaded file: %w: ", err)
	}

	remoteRelease.SHA1 = hex.EncodeToString(hash.Sum(nil))

	return Local{Lock: remoteRelease, LocalPath: filePath}, nil
}

type ReleaseAssetDownloader interface {
	DownloadReleaseAsset(ctx context.Context, owner, repo string, id int64, followRedirectsClient *http.Client) (rc io.ReadCloser, redirectURL string, err error)
}

func findAssetFile(list []*github.ReleaseAsset, lock cargo.BOSHReleaseTarballLock) (*github.ReleaseAsset, bool) {
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
