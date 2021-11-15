package component

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v40/github"
	"golang.org/x/oauth2"

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
func (grs GithubReleaseSource) GetMatchedRelease(s Spec) (Lock, bool, error) {
	// TODO: fail if spec.Version is a multiple value constraint (meaning a range of versions)
	versionArray := strings.Split(s.Version, ".")
	if len(versionArray) < 3 {
		return Lock{}, false, errors.New("shit, man")
	}
	return LockFromGithubRelease(context.TODO(), grs.Client.Repositories, grs.Org, s)
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (grs GithubReleaseSource) FindReleaseVersion(s Spec) (Lock, bool, error) {
	// TODO: fail if spec.Version is a single value constraint (meaning a single version)
	// if spec.Version is a range, find the latest, and make that the version we use in 129.
	return LockFromGithubRelease(context.TODO(), grs.Client.Repositories, grs.Org, s)
}

// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
// It should also calculate and set the SHA1 field on the Local result; it does not need
// to ensure the sums match, the caller must verify this.
func (GithubReleaseSource) DownloadRelease(releasesDir string, remoteRelease Lock) (Local, error) {
	panic("blah")
}

//counterfeiter:generate -o ./fakes/git_hub_repo_api.go --fake-name GitHubRepositoryAPI . GitHubRepositoryAPI

type GitHubRepositoryAPI interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
	DownloadReleaseAsset(ctx context.Context, owner, repo string, id int64, followRedirectsClient *http.Client) (rc io.ReadCloser, redirectURL string, err error)
}

type ErrorUnexpectedStatus struct {
	Want, Got int
}

func checkStatus(want, got int) error {
	if want != got {
		return ErrorUnexpectedStatus{
			Want: want, Got: got,
		}
	}
	return nil
}

func (err ErrorUnexpectedStatus) Error() string {
	return fmt.Sprintf("request responded with %s (%d)",
		http.StatusText(err.Got), err.Got,
	)
}

func OwnerAndRepoFromGitHubURI(urlStr string) (owner, repo string) {
	u, err := url.Parse(urlStr)
	if err != nil {
		if !strings.HasPrefix(urlStr, "git@github.com:") {
			return
		}
		u, err = url.Parse("/" + strings.TrimPrefix(urlStr, "git@github.com:"))
		if err != nil {
			return
		}
		u.Host = "github.com"
	}
	if u.Host != "github.com" {
		return
	}
	if filepath.Ext(u.Path) == ".git" {
		u.Path = strings.TrimSuffix(u.Path, ".git")
	}
	u.Path, repo = path.Split(u.Path)
	_, owner = path.Split(strings.TrimSuffix(u.Path, "/"))
	return owner, repo
}

func LockFromGithubRelease(ctx context.Context, releaseGetter GitHubRepositoryAPI, owner string, spec Spec) (Lock, bool, error) {
	for _, repoURL := range spec.GitRepositories {
		repoOwner, repoName := OwnerAndRepoFromGitHubURI(repoURL)
		if repoOwner != owner || repoName == "" {
			continue
		}
		release, response, err := releaseGetter.GetReleaseByTag(ctx, owner, repoName, spec.Version)
		if err != nil {
			return Lock{}, false, err
		}
		err = checkStatus(http.StatusOK, response.StatusCode)
		if err != nil {
			return Lock{}, false, err
		}
		expectedAssetName := fmt.Sprintf("%s-%s.tgz", spec.Name, spec.Version)
		for _, asset := range release.Assets {
			if asset.GetName() != expectedAssetName {
				continue
			}
			rc, _, err := releaseGetter.DownloadReleaseAsset(ctx, repoOwner, repoName, *asset.ID, http.DefaultClient)
			if err != nil {
				return Lock{}, false, err
			}
			sum, err := calculateSHA1(rc)
			if err != nil {
				return Lock{}, false, err
			}
			return Lock{
				Name:         spec.Name,
				Version:      release.GetTagName(),
				RemoteSource: ReleaseSourceTypeGithub,
				RemotePath:   asset.GetBrowserDownloadURL(),
				SHA1:         sum,
			}, true, nil
		}
	}
	return Lock{}, false, nil
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
