package release

import (
	"bytes"
	_ "embed"
	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v40/github"
	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"io"
	"io/fs"
	"os"
	"regexp"
	"testing"
	"time"
)

//go:embed testdata/runtime-rn.html.md.erb
var releaseNotesPageTAS27 string

func TestParseNotesPage(t *testing.T) {
	please := Ω.NewWithT(t)

	t0, err := time.Parse("2006-01-02", "2021-11-23")
	please.Expect(err).NotTo(Ω.HaveOccurred())

	someDeveloper := &object.Signature{Name: "author", Email: "email", When: t0}

	// setup docs repo with initial state
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	please.Expect(err).NotTo(Ω.HaveOccurred())
	wt, err := repo.Worktree()
	please.Expect(err).NotTo(Ω.HaveOccurred())
	{
		notesMD, err := wt.Filesystem.Create("notes.md")
		_, _ = notesMD.Write([]byte(releaseNotesPageTAS27))
		_ = notesMD.Close()
		_, _ = wt.Add(notesMD.Name())
		_, err = wt.Commit("initial commit", &git.CommitOptions{
			All:       true,
			Author:    someDeveloper,
			Committer: someDeveloper,
		})
		please.Expect(err).NotTo(Ω.HaveOccurred())
	}
	// parse release notes
	page, err := ParseNotesPage(releaseNotesPageTAS27)
	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(page.Releases).To(Ω.HaveLen(42))

	// assign new release notes data
	alreadyPublishedReleaseNotesData := NotesData{
		ReleaseDate: t0,
		Version:     semver.MustParse("2.7.41"),
		Issues: []*github.Issue{
			{Title: strPtr("**[Bug Fix]** Breaking Change: Any customers with gorouter certificates lacking a SubjectAltName extension will experience failures upon deployment. As a workaround to complete deployment while new certificates are procured, enable the \"Enable temporary workaround for certs without SANs\" property in the Networking section of the TAS tile. For more information on updating certs, see https://community.pivotal.io/s/article/Routing-and-golang-1-15-X-509-CommonName-deprecation?language=en_US")},
			{Title: strPtr("**[Bug Fix]** Cloud Controller - Ensure app lifecycle_type is not nil when determining app lifecycle")},
		},
		Stemcell: cargo.Stemcell{
			OS: "ubuntu-xenial", Version: "456.0",
		},
		Components: []ComponentData{
			{Lock: component.Lock{Name: "backup-and-restore-sdk", Version: "1.18.26"}},
			{Lock: component.Lock{Name: "binary-offline-buildpack", Version: "1.0.40"}},
			{Lock: component.Lock{Name: "bosh-dns-aliases", Version: "0.0.3"}},
			{Lock: component.Lock{Name: "bosh-system-metrics-forwarder", Version: "0.0.20"}},
			{Lock: component.Lock{Name: "bpm", Version: "1.1.15"}},
			{Lock: component.Lock{Name: "capi", Version: "1.84.20"}},
			{Lock: component.Lock{Name: "cf-autoscaling", Version: "241"}},
			{Lock: component.Lock{Name: "cf-backup-and-restore", Version: "0.0.11"}},
			{Lock: component.Lock{Name: "cf-cli", Version: "1.32.0"}},
			{Lock: component.Lock{Name: "cf-networking", Version: "2.40.0"}},
			{Lock: component.Lock{Name: "cf-smoke-tests", Version: "40.0.134"}},
			{Lock: component.Lock{Name: "cf-syslog-drain", Version: "10.2.5"}},
			{Lock: component.Lock{Name: "cflinuxfs3", Version: "0.264.0"}},
			{Lock: component.Lock{Name: "credhub", Version: "2.5.13"}},
			{Lock: component.Lock{Name: "diego", Version: "2.53.0"}},
			{Lock: component.Lock{Name: "dotnet-core-offline-buildpack", Version: "2.3.36"}},
			{Lock: component.Lock{Name: "garden-runc", Version: "1.19.30"}},
			{Lock: component.Lock{Name: "go-offline-buildpack", Version: "1.9.37"}},
			{Lock: component.Lock{Name: "haproxy", Version: "9.6.1"}},
			{Lock: component.Lock{Name: "istio", Version: "1.3.0"}},
			{Lock: component.Lock{Name: "java-offline-buildpack", Version: "4.42"}},
			{Lock: component.Lock{Name: "leadership-election", Version: "1.4.2"}},
			{Lock: component.Lock{Name: "log-cache", Version: "2.1.17"}},
			{Lock: component.Lock{Name: "loggregator-agent", Version: "3.21.18"}},
			{Lock: component.Lock{Name: "loggregator", Version: "105.6.8"}},
			{Lock: component.Lock{Name: "mapfs", Version: "1.2.4"}},
			{Lock: component.Lock{Name: "metric-registrar", Version: "1.1.9"}},
			{Lock: component.Lock{Name: "mysql-monitoring", Version: "9.7.0"}},
			{Lock: component.Lock{Name: "nats", Version: "40"}},
			{Lock: component.Lock{Name: "nfs-volume", Version: "2.3.10"}},
			{Lock: component.Lock{Name: "nginx-offline-buildpack", Version: "1.1.32"}},
			{Lock: component.Lock{Name: "nodejs-offline-buildpack", Version: "1.7.63"}},
			{Lock: component.Lock{Name: "notifications-ui", Version: "39"}},
			{Lock: component.Lock{Name: "notifications", Version: "62"}},
			{Lock: component.Lock{Name: "php-offline-buildpack", Version: "4.4.48"}},
			{Lock: component.Lock{Name: "push-apps-manager-release", Version: "670.0.29"}},
			{Lock: component.Lock{Name: "push-usage-service-release", Version: "670.0.36"}},
			{Lock: component.Lock{Name: "pxc", Version: "0.39.0"}},
			{Lock: component.Lock{Name: "python-offline-buildpack", Version: "1.7.47"}},
			{Lock: component.Lock{Name: "r-offline-buildpack", Version: "1.1.23"}},
			{Lock: component.Lock{Name: "routing", Version: "0.226.0"}},
			{Lock: component.Lock{Name: "ruby-offline-buildpack", Version: "1.8.48"}},
			{Lock: component.Lock{Name: "silk", Version: "2.40.0"}},
			{Lock: component.Lock{Name: "smb-volume", Version: "3.0.1"}},
			{Lock: component.Lock{Name: "staticfile-offline-buildpack", Version: "1.5.26"}},
			{Lock: component.Lock{Name: "statsd-injector", Version: "1.11.16"}},
			{Lock: component.Lock{Name: "syslog", Version: "11.7.5"}},
			{Lock: component.Lock{Name: "uaa", Version: "73.4.32"}},
		},
		Bumps: component.BumpList{
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
	please.Expect(err).NotTo(Ω.HaveOccurred())

	err = page.Add(alreadyPublishedVersionNote)
	please.Expect(err).NotTo(Ω.HaveOccurred())

	notesMD, err := wt.Filesystem.Create("notes.md")
	pageContent := new(bytes.Buffer)
	_, err = page.WriteTo(io.MultiWriter(notesMD, pageContent))
	please.Expect(err).NotTo(Ω.HaveOccurred())
	_ = notesMD.Close()
	_, _ = wt.Add(notesMD.Name())
	status, err := wt.Status()
	please.Expect(err).NotTo(Ω.HaveOccurred())
	if !status.IsClean() {
		_ = os.WriteFile("exp.txt", []byte(releaseNotesPageTAS27), fs.ModePerm)
		_ = os.WriteFile("got.txt", []byte(pageContent.String()), fs.ModePerm)
		t.Logf("run: %q", "diff --unified internal/release/{exp,got}.txt | colordiff")
	}
	please.Expect(status.IsClean()).To(Ω.BeTrue())
}

func TestParseNotesPageWithExpressionAndReleasesSentinel(t *testing.T) {
	const testReleasesSentinel = "releases:"
	var exp = regexp.MustCompile(`(?m)(?P<notes>r(?P<version>\d+)\.*)`).String()

	t.Run("multiple releases", func(t *testing.T) {
		please := Ω.NewWithT(t)

		input := "prefix.releases:r1.r2..r3r4...r5...suffix"

		page, err := ParseNotesPageWithExpressionAndReleasesSentinel(input, exp, testReleasesSentinel)
		please.Expect(err).NotTo(Ω.HaveOccurred())

		please.Expect(page.Releases).To(Ω.HaveLen(5))
		please.Expect(page.Releases).To(Ω.Equal([]VersionNote{
			{Version: "1", Notes: "r1."},
			{Version: "2", Notes: "r2.."},
			{Version: "3", Notes: "r3"},
			{Version: "4", Notes: "r4..."},
			{Version: "5", Notes: "r5..."},
		}))

		please.Expect(page.Prefix).To(Ω.Equal("prefix.releases:"))
		please.Expect(page.Suffix).To(Ω.Equal("suffix"))
	})

	t.Run("no releases", func(t *testing.T) {
		please := Ω.NewWithT(t)

		input := "prefix.releases:suffix"

		page, err := ParseNotesPageWithExpressionAndReleasesSentinel(input, exp, testReleasesSentinel)
		please.Expect(err).NotTo(Ω.HaveOccurred())

		please.Expect(page.Releases).To(Ω.HaveLen(0))
		please.Expect(page.Prefix).To(Ω.Equal("prefix.releases:"))
		please.Expect(page.Suffix).To(Ω.Equal("suffix"))
	})
}

func TestReleaseNotesPage_Add(t *testing.T) {
	// I imagine if the function signature for Add also returned an integer
	//   func (page NotesPage) Add(note VersionNote) (int, error)
	// it would be easier to test. It also would make logging where in the document
	// release was added easier.
	// For example, the release-notes could log something like:
	//   The release notes for tile 2.7.43 were inserted at the top of the document (index 0)

	t.Fail()
	t.Run("initial release", func(t *testing.T) {
		please := Ω.NewWithT(t)
		page := NotesPage{
			Exp: regexp.MustCompile(`r\d+`), // a simpler release expression
		}
		note := VersionNote{Version: "1", Notes: "r1"}
		err := page.Add(note)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(page.Releases).To(Ω.ConsistOf(note))
	})
	t.Run("new latest release", func(t *testing.T) {
		t.Skip()
		// TODO: New release is newer than all in .Releases
	})
	t.Run("update existing release notes", func(t *testing.T) {
		t.Skip()
		// TODO: matches existing version
	})
	t.Run("update existing release notes", func(t *testing.T) {
		t.Skip()
		// TODO: matches existing version
	})
	t.Run("insert between", func(t *testing.T) {
		t.Skip()
		// TODO: less than first but greater than last
	})
	t.Run("notes version field is invalid", func(t *testing.T) {
		t.Skip()
		// TODO: less than first but greater than last
	})
	t.Run("add notes to end", func(t *testing.T) {
		t.Skip()
		// TODO: version is less than last
	})
	t.Run("notes content does not match page regex", func(t *testing.T) {
		t.Skip()
		// TODO: version is less than last
	})
}

func TestReleaseNotesPage_WriteTo(t *testing.T) {
	var _ io.WriterTo = (*NotesPage)(nil)
	// TODO ensure expected output
}
