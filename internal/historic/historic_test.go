package historic

import (
	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

func TestVersion(t *testing.T) {
	please := Ω.NewWithT(t)

	// START setup
	tileDir := "tile"
	repo, _ := git.Init(memory.NewStorage(), memfs.New())
	initialHash := commit(t, repo, "alpha release", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "version")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte("1.0.0-alpha.1\n"))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	})
	_ = commit(t, repo, "some other change", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "base.yml")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte("---\nname: something\n"))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	})
	finalHash := commit(t, repo, "ga release", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "version")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte("1.0.0\n"))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	})
	// END setup

	t.Run("alpha", func(t *testing.T) {
		version, err := Version(repo, initialHash, "tile")

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(version).To(Ω.Equal("1.0.0-alpha.1"))
	})

	t.Run("ga release", func(t *testing.T) {
		version, err := Version(repo, finalHash, "tile")

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(version).To(Ω.Equal("1.0.0"))
	})
}

func TestKilnfileLock(t *testing.T) {
	please := Ω.NewWithT(t)

	// START setup
	tileDir := "tile"
	repo, _ := git.Init(memory.NewStorage(), memfs.New())
	initialHash := commit(t, repo, "initial", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "assets.lock")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte(initialKilnfileLock))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	})
	_ = commit(t, repo, "some other change", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "base.yml")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte("---\nname: something\n"))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	})
	badYAML := commit(t, repo, "add some non-yaml", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "Kilnfile.lock")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte(`{{ if eq tile "ert"}}# this is ERT{{}}\n` + finalKilnfileLock))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	})
	finalHash := commit(t, repo, "fix bad yaml", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "Kilnfile.lock")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte(finalKilnfileLock))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	})
	// END setup

	t.Run("legacy bill of materials", func(t *testing.T) {
		kf, err := KilnfileLock(repo, initialHash, "tile")

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(kf.Releases).To(Ω.Equal([]cargo.ComponentLock{
			{Name: "banana", Version: "0.1.0"},
			{Name: "lemon", Version: "1.1.0"},
		}))
	})

	t.Run("kilnfile.lock", func(t *testing.T) {
		finalKF, err := KilnfileLock(repo, finalHash, "tile/Kilnfile")

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(finalKF.Releases).To(Ω.Equal([]cargo.ComponentLock{
			{Name: "banana", Version: "0.9.0"},
			{Name: "lemon", Version: "1.9.0"},
			{Name: "apple", Version: "0.0.1"},
		}))
	})

	t.Run("bad yaml", func(t *testing.T) {
		_, err := KilnfileLock(repo, badYAML, "tile")

		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("cannot unmarshal")))
	})
}

const (
	initialKilnfileLock = `---
releases:
- name: banana
  version: 0.1.0
- name: lemon
  version: 1.1.0
`

	finalKilnfileLock = `---
releases:
- name: banana
  version: 0.9.0
- name: lemon
  version: 1.9.0
- name: apple
  version: 0.0.1
`
)

func commit(t *testing.T, repo *git.Repository, msg string, fn func(wt *git.Worktree) error) plumbing.Hash {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	err = fn(wt)
	if err != nil {
		t.Fatal(err)
	}
	signature := &object.Signature{
		Name:  "releen",
		Email: "releen@example.com",
		When:  time.Unix(1635975074, 0),
	}
	h, err := wt.Commit(msg, &git.CommitOptions{
		Author: signature, Committer: signature,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
