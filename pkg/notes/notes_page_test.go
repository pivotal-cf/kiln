package notes

import (
	"bytes"
	_ "embed"
	"io"
	"io/fs"
	"os"
	"regexp"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v65/github"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

//go:embed testdata/runtime-rn.html.md.erb
var releaseNotesPageTAS27 string

func TestParseNotesPage(t *testing.T) {
	please := NewWithT(t)

	t0, err := time.Parse("2006-01-02", "2021-11-23")
	please.Expect(err).NotTo(HaveOccurred())

	someDeveloper := &object.Signature{Name: "author", Email: "email", When: t0}

	// setup docs repo with initial state
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	please.Expect(err).NotTo(HaveOccurred())
	wt, err := repo.Worktree()
	please.Expect(err).NotTo(HaveOccurred())
	{
		notesMD, _ := wt.Filesystem.Create("notes.md")
		_, _ = notesMD.Write([]byte(releaseNotesPageTAS27))
		_ = notesMD.Close()
		_, _ = wt.Add(notesMD.Name())
		_, err = wt.Commit("initial commit", &git.CommitOptions{
			All:       true,
			Author:    someDeveloper,
			Committer: someDeveloper,
		})
		please.Expect(err).NotTo(HaveOccurred())
	}
	// parse release notes
	page, err := ParsePage(releaseNotesPageTAS27)
	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(page.Releases).To(HaveLen(42))

	// assign new release notes data
	alreadyPublishedReleaseNotesData := Data{
		ReleaseDate: t0,
		Version:     semver.MustParse("2.7.41"),
		Issues: []*github.Issue{
			{Title: strPtr("**[Bug Fix]** Breaking Change: Any customers with gorouter certificates lacking a SubjectAltName extension will experience failures upon deployment. As a workaround to complete deployment while new certificates are procured, enable the \"Enable temporary workaround for certs without SANs\" property in the Networking section of the TAS tile. For more information on updating certs, see https://community.pivotal.io/s/article/Routing-and-golang-1-15-X-509-CommonName-deprecation?language=en_US")},
			{Title: strPtr("**[Bug Fix]** Cloud Controller - Ensure app lifecycle_type is not nil when determining app lifecycle")},
		},
		TrainstatNotes: []string{
			"* **[Feature]** this is a feature.",
			"* **[Bug Fix]** this is a bug fix.",
		},
		Stemcell: cargo.Stemcell{
			OS: "ubuntu-xenial", Version: "456.0",
		},
		Components: []BOSHReleaseData{
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "backup-and-restore-sdk", Version: "1.18.26"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "binary-offline-buildpack", Version: "1.0.40"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "bosh-dns-aliases", Version: "0.0.3"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "bosh-system-metrics-forwarder", Version: "0.0.20"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "bpm", Version: "1.1.15"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "capi", Version: "1.84.20"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "cf-autoscaling", Version: "241"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "cf-backup-and-restore", Version: "0.0.11"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "cf-cli", Version: "1.32.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "cf-networking", Version: "2.40.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "cf-smoke-tests", Version: "40.0.134"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "cf-syslog-drain", Version: "10.2.5"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "cflinuxfs3", Version: "0.264.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "credhub", Version: "2.5.13"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "diego", Version: "2.53.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "dotnet-core-offline-buildpack", Version: "2.3.36"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "garden-runc", Version: "1.19.30"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "go-offline-buildpack", Version: "1.9.37"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "haproxy", Version: "9.6.1"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "istio", Version: "1.3.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "java-offline-buildpack", Version: "4.42"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "leadership-election", Version: "1.4.2"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "log-cache", Version: "2.1.17"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "loggregator-agent", Version: "3.21.18"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "loggregator", Version: "105.6.8"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "mapfs", Version: "1.2.4"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "metric-registrar", Version: "1.1.9"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "mysql-monitoring", Version: "9.7.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "nats", Version: "40"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "nfs-volume", Version: "2.3.10"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "nginx-offline-buildpack", Version: "1.1.32"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "nodejs-offline-buildpack", Version: "1.7.63"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "notifications-ui", Version: "39"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "notifications", Version: "62"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "php-offline-buildpack", Version: "4.4.48"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "push-apps-manager-release", Version: "670.0.29"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "push-usage-service-release", Version: "670.0.36"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "pxc", Version: "0.39.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "python-offline-buildpack", Version: "1.7.47"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "r-offline-buildpack", Version: "1.1.23"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "routing", Version: "0.226.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "ruby-offline-buildpack", Version: "1.8.48"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "silk", Version: "2.40.0"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "smb-volume", Version: "3.0.1"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "staticfile-offline-buildpack", Version: "1.5.26"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "statsd-injector", Version: "1.11.16"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "syslog", Version: "11.7.5"}},
			{BOSHReleaseTarballLock: cargo.BOSHReleaseTarballLock{Name: "uaa", Version: "73.4.32"}},
		},
		Bumps: cargo.BumpList{
			{Name: "backup-and-restore-sdk", ToVersion: "1.18.26"},
			{Name: "bpm", ToVersion: "1.1.15"},
			{Name: "capi", ToVersion: "1.84.20"},
			{Name: "cf-autoscaling", ToVersion: "241"},
			{Name: "cf-networking", ToVersion: "2.40.0"},
			{Name: "cflinuxfs3", ToVersion: "0.264.0"},
			{Name: "dotnet-core-offline-buildpack", ToVersion: "2.3.36"},
			{Name: "go-offline-buildpack", ToVersion: "1.9.37"},
			{Name: "nodejs-offline-buildpack", ToVersion: "1.7.63"},
			{Name: "php-offline-buildpack", ToVersion: "4.4.48"},
			{Name: "python-offline-buildpack", ToVersion: "1.7.47"},
			{Name: "r-offline-buildpack", ToVersion: "1.1.23"},
			{Name: "routing", ToVersion: "0.226.0"},
			{Name: "ruby-offline-buildpack", ToVersion: "1.8.48"},
			{Name: "silk", ToVersion: "2.40.0"},
			{Name: "staticfile-offline-buildpack", ToVersion: "1.5.26"},
		},
	}

	alreadyPublishedVersionNote, err := alreadyPublishedReleaseNotesData.WriteVersionNotes()
	please.Expect(err).NotTo(HaveOccurred())

	err = page.Add(alreadyPublishedVersionNote)
	please.Expect(err).NotTo(HaveOccurred())

	notesMD, _ := wt.Filesystem.Create("notes.md")
	pageContent := new(bytes.Buffer)
	_, err = page.WriteTo(io.MultiWriter(notesMD, pageContent))
	please.Expect(err).NotTo(HaveOccurred())
	_ = notesMD.Close()
	_, _ = wt.Add(notesMD.Name())
	status, err := wt.Status()
	please.Expect(err).NotTo(HaveOccurred())
	if !status.IsClean() {
		_ = os.WriteFile("exp.txt", []byte(releaseNotesPageTAS27), fs.ModePerm)
		_ = os.WriteFile("got.txt", pageContent.Bytes(), fs.ModePerm)
		t.Logf("run: %q", "diff --unified internal/release/{exp,got}.txt | colordiff")
	}
	please.Expect(status.IsClean()).To(BeTrue())
}

func TestParseNotesPageWithExpressionAndReleasesSentinel(t *testing.T) {
	const testReleasesSentinel = "releases:"
	exp := regexp.MustCompile(`(?m)(?P<notes>r(?P<version>\d+)\.*)`).String()

	t.Run("multiple releases", func(t *testing.T) {
		please := NewWithT(t)

		input := "prefix.releases:r1.r2..r3r4...r5...suffix"

		page, err := ParsePageWithExpressionAndReleasesSentinel(input, exp, testReleasesSentinel)
		please.Expect(err).NotTo(HaveOccurred())

		please.Expect(page.Releases).To(HaveLen(5))
		please.Expect(page.Releases).To(Equal([]TileRelease{
			{Version: "1", Notes: "r1."},
			{Version: "2", Notes: "r2.."},
			{Version: "3", Notes: "r3"},
			{Version: "4", Notes: "r4..."},
			{Version: "5", Notes: "r5..."},
		}))

		please.Expect(page.Prefix).To(Equal("prefix.releases:"))
		please.Expect(page.Suffix).To(Equal("suffix"))
	})

	t.Run("no releases", func(t *testing.T) {
		please := NewWithT(t)

		input := "prefix.releases:suffix"

		page, err := ParsePageWithExpressionAndReleasesSentinel(input, exp, testReleasesSentinel)
		please.Expect(err).NotTo(HaveOccurred())

		please.Expect(page.Releases).To(HaveLen(0))
		please.Expect(page.Prefix).To(Equal("prefix.releases:"))
		please.Expect(page.Suffix).To(Equal("suffix"))
	})
}

func Test_newFetchNotesData(t *testing.T) {
	t.Run("when called", func(t *testing.T) {
		please := NewWithT(t)
		f, err := newFetchNotesData(&git.Repository{}, "o", "r", "k", "ri", "rf", nil, IssuesQuery{
			IssueMilestone: "BLA",
		}, &TrainstatClient{
			host: "test",
		})
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(f.repoOwner).To(Equal("o"))
		please.Expect(f.repoName).To(Equal("r"))
		please.Expect(f.kilnfilePath).To(Equal("k"))
		please.Expect(f.initialRevision).To(Equal("ri"))
		please.Expect(f.finalRevision).To(Equal("rf"))
		please.Expect(f.historicKilnfile).NotTo(BeNil())
		please.Expect(f.historicVersion).NotTo(BeNil())
		please.Expect(f.issuesQuery).To(Equal(IssuesQuery{
			IssueMilestone: "BLA",
		}))
		please.Expect(f.trainstatClient).To(Equal(&TrainstatClient{
			host: "test",
		}))
	})
	t.Run("when repo is nil", func(t *testing.T) {
		please := NewWithT(t)
		_, err := newFetchNotesData(nil, "o", "r", "k", "ri", "rf", &github.Client{}, IssuesQuery{}, &TrainstatClient{})
		please.Expect(err).To(HaveOccurred())
	})
	t.Run("when repo is not nil", func(t *testing.T) {
		please := NewWithT(t)
		f, err := newFetchNotesData(&git.Repository{
			Storer: &memory.Storage{},
		}, "o", "r", "k", "ri", "rf", &github.Client{}, IssuesQuery{}, &TrainstatClient{})
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(f.repository).NotTo(BeNil())
		please.Expect(f.revisionResolver).NotTo(BeNil())
		please.Expect(f.Storer).NotTo(BeNil())
	})
	t.Run("when github client is not nil", func(t *testing.T) {
		please := NewWithT(t)
		f, err := newFetchNotesData(&git.Repository{}, "o", "r", "k", "ri", "rf", &github.Client{
			Issues:       &github.IssuesService{},
			Repositories: &github.RepositoriesService{},
		}, IssuesQuery{}, &TrainstatClient{})
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(f.issuesService).NotTo(BeNil())
		please.Expect(f.releasesService).NotTo(BeNil())
	})
	t.Run("when github client is nil", func(t *testing.T) {
		please := NewWithT(t)
		f, err := newFetchNotesData(&git.Repository{}, "o", "r", "k", "ri", "rf", nil, IssuesQuery{}, &TrainstatClient{})
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(f.issuesService).To(BeNil())
		please.Expect(f.releasesService).To(BeNil())
	})
}

func TestReleaseNotesPage_Add(t *testing.T) {
	// I imagine if the function signature for Add also returned an integer
	//   func (page Page) Add(note TileRelease) (int, error)
	// it would be easier to test. It also would make logging where in the document
	// release was added easier.
	// For example, the release-notes could log something like:
	//   The release notes for tile 2.7.43 were inserted at the top of the document (index 0)

	t.Run("initial release", func(t *testing.T) {
		please := NewWithT(t)
		page := Page{
			Exp: regexp.MustCompile(`r\d+`), // a simpler release expression
		}
		note := TileRelease{Version: "1", Notes: "r1"}
		err := page.Add(note)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(page.Releases).To(ConsistOf(note))
	})
	t.Run("new latest release", func(t *testing.T) {
		please := NewWithT(t)
		page := Page{
			Exp: regexp.MustCompile(`r\d+`),
			Releases: []TileRelease{
				{Version: "2", Notes: "r2"},
			},
		}
		newNote := TileRelease{Version: "3", Notes: "r3"}
		err := page.Add(newNote)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(page.Releases).To(Equal([]TileRelease{
			{Version: "3", Notes: "r3"},
			{Version: "2", Notes: "r2"},
		}))
	})
	t.Run("update existing release notes", func(t *testing.T) {
		please := NewWithT(t)
		page := Page{
			Exp: regexp.MustCompile(`r\d+`),
			Releases: []TileRelease{
				{Version: "1", Notes: "r1"},
			},
		}
		newNote := TileRelease{Version: "1", Notes: "r2"}
		err := page.Add(newNote)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(page.Releases).To(Equal([]TileRelease{
			{Version: "1", Notes: "r2"},
		}))
	})
	t.Run("insert between", func(t *testing.T) {
		please := NewWithT(t)
		page := Page{
			Exp: regexp.MustCompile(`r\d+`),
			Releases: []TileRelease{
				{Version: "3", Notes: "r3"},
				{Version: "1", Notes: "r1"},
			},
		}
		newNote := TileRelease{Version: "2", Notes: "r2"}
		err := page.Add(newNote)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(page.Releases).To(Equal([]TileRelease{
			{Version: "3", Notes: "r3"},
			{Version: "2", Notes: "r2"},
			{Version: "1", Notes: "r1"},
		}))
	})
	t.Run("notes version field is invalid", func(t *testing.T) {
		please := NewWithT(t)
		page := Page{
			Exp: regexp.MustCompile(`r\d+`),
			Releases: []TileRelease{
				{Version: "1", Notes: "r1"},
			},
		}
		newNote := TileRelease{Version: "s", Notes: "r2"}
		err := page.Add(newNote)
		please.Expect(err).To(HaveOccurred())
	})
	t.Run("add notes to end", func(t *testing.T) {
		please := NewWithT(t)
		page := Page{
			Exp: regexp.MustCompile(`r\d+`),
			Releases: []TileRelease{
				{Version: "3", Notes: "r3"},
				{Version: "2", Notes: "r2"},
			},
		}
		newNote := TileRelease{Version: "1", Notes: "r1"}
		err := page.Add(newNote)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(page.Releases).To(Equal([]TileRelease{
			{Version: "3", Notes: "r3"},
			{Version: "2", Notes: "r2"},
			{Version: "1", Notes: "r1"},
		}))
	})
	t.Run("notes content does not match page regex", func(t *testing.T) {
		please := NewWithT(t)
		page := Page{
			Exp: regexp.MustCompile(`r\d+`),
			Releases: []TileRelease{
				{Version: "1", Notes: "r1"},
			},
		}
		newNote := TileRelease{Version: "2", Notes: "s2"}
		err := page.Add(newNote)
		please.Expect(err).To(HaveOccurred())
	})
}

func TestReleaseNotesPage_WriteTo(t *testing.T) {
	var _ io.WriterTo = (*Page)(nil)
}
