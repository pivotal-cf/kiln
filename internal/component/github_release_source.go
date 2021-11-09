package component

import (
	"context"
	"fmt"
	"github.com/google/go-github/v40/github"
	"golang.org/x/oauth2"
	"log"
	"os"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type GHRequirement struct {
	RepoNames   []string
	Releases    []string
	TarballPath string
}

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
func (grs GithubReleaseSource) GetMatchedRelease(req Spec) (Lock, bool, error) {
	return LockFromGithubRelease(context.TODO(), grs.Client.Repositories, grs.ID, req)
}

//counterfeiter:generate -o ./fakes/get_release_by_tagger.go --fake-name GetReleaseByTagger . GetReleaseByTagger

type GetReleaseByTagger interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
}

func LockFromGithubRelease(ctx context.Context, repo GetReleaseByTagger, owner string, spec Spec) (Lock, bool, error) {
	//getRepoAndOwner := func(string) (owner, repo string) { return } //  can use url.Parse and strings.Split
	//
	//for _, repoURL := range spec.Repositories {
	//	o, r := getRepoAndOwner(repoURL)
	//	if o != owner || r == "" {
	//		continue
	//	}
	//	release, _, _ := repo.GetReleaseByTag(ctx, owner, r, spec.Version)  // TODO: handle error
	//	return Lock{
	//		Version: release.GetTagName(),
	//	}, false, nil
	//}

	release, _, _ := repo.GetReleaseByTag(ctx, owner, "", "")
	return Lock{
		Version: release.GetTagName(),
	}, false, nil // return error?
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (GithubReleaseSource) FindReleaseVersion(Spec) (Lock, bool, error) {
	panic("blah")
}

// DownloadRelease downloads the release and writes the resulting file to the releasesDir.
// It should also calculate and set the SHA1 field on the Local result; it does not need
// to ensure the sums match, the caller must verify this.
func (GithubReleaseSource) DownloadRelease(releasesDir string, remoteRelease Lock) (Local, error) {
	panic("blah")
}

func (grs GithubReleaseSource) ListAllOfTheCrap(ctx context.Context, org string) {
	var allRepos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{Sort: "name"}
	for {
		repos, resp, err := grs.Client.Repositories.ListByOrg(ctx, "github", opt)
		if err != nil {
			panic("bad crap: " + err.Error())
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	for _, r := range allRepos {
		fmt.Println(r.GetName())
	}
}
