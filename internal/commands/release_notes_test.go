package commands

import (
	"bytes"
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/onsi/gomega"

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

func TestReleaseNotes_updateDocsFile(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		please := NewWithT(t)

		mustParseTime := func(tm time.Time, err error) time.Time {
			if err != nil {
				t.Fatal(err)
			}
			return tm
		}

		tmpDir, err := os.MkdirTemp("", "test-release-notes-updated-docs")
		please.Expect(err).NotTo(HaveOccurred())

		docsFile := filepath.Join(tmpDir, "docs")
		docsFileContent := `
---
title: TAS for VMs v7.0 Release notes
owner: Release Engineering
---

These are the release notes for <%= vars.app_runtime_first %> <%= vars.v_major_version %>.

<%= vars.app_runtime_abbr %> is certified by the Cloud Foundry Foundation for 2024.

For more information about the Cloud Foundry Certified Provider Program, see [How Do I Become a Certified
Provider?](https://www.cloudfoundry.org/certified-platforms-how-to/) on the Cloud Foundry website.

Because VMware uses the Percona Distribution for MySQL, expect a time lag between Oracle releasing a MySQL patch and VMware releasing
<%= vars.app_runtime_abbr %> containing that patch.

<hr>

**Deprecation Notice:** Cloud Foundry Command-Line Interface (cf CLI) v7 will be deprecated and will lose support starting next release. For how to upgrade to cf CLI v8 see [Upgrading to cf CLI
v8](../cf-cli/v8.html).

>**Important**
>For release 6.0.0 to 6.0.3, CVE-2024-22279. (VMware Tanzu Application Service for VMs GoRouter contains an RFC protocol
>issue that can lead to a denial of service) has been fixed. For details, see
>[TNZ-2024-0100](https://support.broadcom.com/web/ecx/support-content-notification/-/external/content/SecurityAdvisories/0/24486).

## <a id='releases'></a> Releases

### <a id='10.2.0'></a> 10.2.0

**Release Date:** 03/11/2025

* **[Feature]** In the "Networking" tab, the "Overlay subnet" property is now the "Overlay subnets" property. Previously you could only provide one CIDR, now you can provide a list of comma-separated list of CIDRs.
* **[Feature]** Operators can now "Enable comma-delimited lists of IPs for application security group (ASG) destinations" via the property in the "Networking

`

		err = os.WriteFile(docsFile, []byte(docsFileContent), 0o644)
		please.Expect(err).NotTo(HaveOccurred())

		data := notes.Data{
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
		}

		rn := ReleaseNotes{}
		rn.Options.DocsFile = docsFile
		rn.readFile = os.ReadFile

		err = rn.updateDocsFile(data)
		please.Expect(err).ToNot(HaveOccurred())
		contents, err := os.ReadFile(rn.Options.DocsFile)
		please.Expect(contents).To(ContainSubstring("**[Feature Improvement]** Reduce default log-cache max per source"))

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

		rn := ReleaseNotes{}
		rn.repository = repo
		initErr := rn.initRepo()
		please.Expect(initErr).NotTo(HaveOccurred())
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
				"git@github.com:releen/kiln.git",
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
