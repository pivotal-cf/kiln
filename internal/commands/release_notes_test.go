package commands

import (
	"bytes"
	"context"
	_ "embed"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v50/github"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/notes"
)

var _ jhanda.Command = ReleaseNotes{}

func TestReleaseNotes_Usage(t *testing.T) {
	please := NewWithT(t)

	rn := ReleaseNotes{}

	please.Expect(rn.Usage().Description).NotTo(BeEmpty())
	please.Expect(rn.Usage().ShortDescription).NotTo(BeEmpty())
	please.Expect(rn.Usage().Flags).NotTo(BeNil())
}

//go:embed testdata/release_notes_output.md
var releaseNotesExpectedOutput string

func TestReleaseNotes_Execute(t *testing.T) {
	t.Run("when writing to standard out", func(t *testing.T) {
		mustParseTime := func(tm time.Time, err error) time.Time {
			if err != nil {
				t.Fatal(err)
			}
			return tm
		}

		please := NewWithT(t)

		nonNilRepo, _ := git.Init(memory.NewStorage(), memfs.New())
		please.Expect(nonNilRepo).NotTo(BeNil())

		readFileCount := 0
		readFileFunc := func(string) ([]byte, error) {
			readFileCount++
			return nil, nil
		}

		var (
			tileRepoHost, tileRepoOwner, tileRepoName, kilnfilePath, initialRevision, finalRevision string

			issuesQuery notes.IssuesQuery
			repository  *git.Repository
			client      *github.Client
			ctx         context.Context

			out bytes.Buffer
		)
		rn := ReleaseNotes{
			Writer:     &out,
			repository: nonNilRepo,
			repoHost:   "github.com",
			repoOwner:  "bunch",
			repoName:   "banana",
			readFile:   readFileFunc,
			fetchNotesData: func(c context.Context, repo *git.Repository, ghc *github.Client, trh, tro, trn, kfp, ir, fr string, iq notes.IssuesQuery, _ notes.TrainstatNotesFetcher, __ map[string]any) (notes.Data, error) {
				ctx, repository, client = c, repo, ghc
				tileRepoHost, tileRepoOwner, tileRepoName, kilnfilePath, initialRevision, finalRevision = trh, tro, trn, kfp, ir, fr
				issuesQuery = iq
				return notes.Data{
					ReleaseDate: mustParseTime(time.Parse(releaseDateFormat, "2021-11-04")),
					Version:     semver.MustParse("0.1.0"),
					Issues: []*github.Issue{
						{Title: strPtr("**[Feature Improvement]** Reduce default log-cache max per source")},
						{Title: strPtr("**[Bug Fix]** banana metadata migration does not fail on upgrade from previous LTS")},
					},
					Stemcell: cargo.Stemcell{
						OS: "fruit-tree", Version: "40000.2",
					},
					Components: []notes.BOSHReleaseData{
						{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "banana", Version: "1.2.0"}, Releases: []*github.RepositoryRelease{
							{TagName: strPtr("1.2.0"), Body: strPtr("peal\nis\nyellow")},
							{TagName: strPtr("1.1.1"), Body: strPtr("remove from bunch")},
						}},
						{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "lemon", Version: "1.1.0"}},
					},
					Bumps: cargo.BumpList{
						{Name: "banana", From: cargo.BOSHReleaseTarballLock{Version: "1.1.0"}, To: cargo.BOSHReleaseTarballLock{Version: "1.2.0"}},
					},
					TrainstatNotes: []string{
						"* **[Feature]** this is a feature.",
						"* **[Bug Fix]** this is a bug fix.",
					},
				}, nil
			},
		}

		rn.Options.GithubAccessToken = "secret"

		err := rn.Execute([]string{
			"--kilnfile=tile/Kilnfile",
			"--release-date=2021-11-04",
			"--github_access_token=lemon",
			"--github-issue-milestone=smoothie",
			"--github-issue-label=tropical",
			"--github-issue=54000",
			"--github-issue-label=20000",
			"--github-issue=54321",
			"tile/1.1.0",
			"tile/1.2.0",
		})
		please.Expect(err).NotTo(HaveOccurred())

		please.Expect(ctx).NotTo(BeNil())
		please.Expect(repository).NotTo(BeNil())
		please.Expect(client).NotTo(BeNil())

		please.Expect(tileRepoHost).To(Equal("github.com"))
		please.Expect(tileRepoOwner).To(Equal("bunch"))
		please.Expect(tileRepoName).To(Equal("banana"))
		please.Expect(kilnfilePath).To(Equal("tile/Kilnfile"))
		please.Expect(initialRevision).To(Equal("tile/1.1.0"))
		please.Expect(finalRevision).To(Equal("tile/1.2.0"))

		please.Expect(issuesQuery.IssueMilestone).To(Equal("smoothie"))
		please.Expect(issuesQuery.IssueIDs).To(Equal([]string{"54000", "54321"}))
		please.Expect(issuesQuery.IssueLabels).To(Equal([]string{"tropical", "20000"}))

		// t.Log(out.String())
		please.Expect(out.String()).To(Equal(releaseNotesExpectedOutput))
	})
}

func TestReleaseNotes_checkInputs(t *testing.T) {
	t.Parallel()

	t.Run("missing args", func(t *testing.T) {
		please := NewWithT(t)

		rn := ReleaseNotes{}
		err := rn.checkInputs(nil)
		please.Expect(err).To(MatchError(ContainSubstring("expected two arguments")))
	})

	t.Run("missing arg", func(t *testing.T) {
		please := NewWithT(t)

		rn := ReleaseNotes{}
		err := rn.checkInputs([]string{"some-hash"})
		please.Expect(err).To(MatchError(ContainSubstring("expected two arguments")))
	})

	t.Run("too many args", func(t *testing.T) {
		please := NewWithT(t)

		rn := ReleaseNotes{}
		err := rn.checkInputs([]string{"a", "b", "c"})
		please.Expect(err).To(MatchError(ContainSubstring("expected two arguments")))
	})

	t.Run("too many args", func(t *testing.T) {
		please := NewWithT(t)

		rn := ReleaseNotes{}
		err := rn.checkInputs([]string{"a", "b", "c"})
		please.Expect(err).To(MatchError(ContainSubstring("expected two arguments")))
	})

	t.Run("bad issue title expression", func(t *testing.T) {
		please := NewWithT(t)

		rn := ReleaseNotes{}
		rn.Options.IssueTitleExp = `\`
		err := rn.checkInputs([]string{"a", "b"})
		please.Expect(err).To(MatchError(ContainSubstring("expression")))
	})

	t.Run("malformed release date", func(t *testing.T) {
		please := NewWithT(t)

		rn := ReleaseNotes{}
		rn.Options.ReleaseDate = `some-date`
		rn.Options.GithubAccessToken = "test-token"
		err := rn.checkInputs([]string{"a", "b"})
		please.Expect(err).To(MatchError(ContainSubstring("cannot parse")))
	})

	t.Run("issue flag without auth", func(t *testing.T) {
		t.Run("milestone", func(t *testing.T) {
			please := NewWithT(t)

			rn := ReleaseNotes{}
			err := rn.checkInputs([]string{"a", "b"})
			please.Expect(err).To(MatchError(ContainSubstring("github_access_token")))
			please.Expect(err).To(MatchError(ContainSubstring("github_enterprise_access_token")))
		})

		t.Run("exp", func(t *testing.T) {
			please := NewWithT(t)

			rn := ReleaseNotes{}
			rn.Options.IssueTitleExp = "s"
			rn.Options.GithubEnterpriseAccessToken = "test-token"
			err := rn.checkInputs([]string{"a", "b"})
			please.Expect(err).NotTo(HaveOccurred())
		})
	})
}

func Test_getGithubRemoteRepoOwnerAndName(t *testing.T) {
	t.Parallel()
	t.Run("when there is a github http remote", func(t *testing.T) {
		please := NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"https://github.com/pivotal-cf/kiln",
			},
		})
		h, o, r, err := getGithubRemoteHostRepoOwnerAndName(repo)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(h).To(Equal("github.com"))
		please.Expect(o).To(Equal("pivotal-cf"))
		please.Expect(r).To(Equal("kiln"))
	})

	t.Run("when there is a github ssh remote", func(t *testing.T) {
		please := NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"git@github.com:pivotal-cf/kiln.git",
			},
		})
		h, o, r, err := getGithubRemoteHostRepoOwnerAndName(repo)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(h).To(Equal("github.com"))
		please.Expect(o).To(Equal("pivotal-cf"))
		please.Expect(r).To(Equal("kiln"))
	})

	t.Run("when there are no remotes", func(t *testing.T) {
		please := NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _, _, err := getGithubRemoteHostRepoOwnerAndName(repo)
		please.Expect(err).To(MatchError(ContainSubstring("not found")))
	})

	t.Run("when there are many remotes", func(t *testing.T) {
		please := NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "fork",
			URLs: []string{
				"git@github.com:crhntr/kiln.git",
			},
		})
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"git@github.com:pivotal-cf/kiln.git",
			},
		})
		h, o, _, err := getGithubRemoteHostRepoOwnerAndName(repo)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(h).To(Equal("github.com"))
		please.Expect(o).To(Equal("pivotal-cf"), "it uses the remote with name 'origin'")
	})
}

func strPtr(s string) *string { return &s }
