package history

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestVersion(t *testing.T) {
	please := NewWithT(t)

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
	change := commit(t, repo, "some other change", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "base.yml")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte("---\nname: something\n"))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	}, initialHash)
	finalHash := commit(t, repo, "ga release", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "version")
		kf, _ := wt.Filesystem.Create(p)
		_, _ = kf.Write([]byte("1.0.0\n"))
		_ = kf.Close()
		_, _ = wt.Add(p)
		return nil
	}, change)
	// END setup

	t.Run("alpha", func(t *testing.T) {
		version, err := Version(repo.Storer, initialHash, "tile")

		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(version).To(Equal("1.0.0-alpha.1"))
	})

	t.Run("ga release", func(t *testing.T) {
		version, err := Version(repo.Storer, finalHash, "tile")

		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(version).To(Equal("1.0.0"))
	})
}

func TestKilnfile(t *testing.T) {
	// START setup
	tileDir := "tile"
	repo, _ := git.Init(memory.NewStorage(), memfs.New())
	initialHashWithoutKilnfile := commit(t, repo, "initial", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "test")
		f, _ := wt.Filesystem.Create(p)
		_, _ = f.Write([]byte(initialKilnfileLock))
		_ = f.Close()
		_, _ = wt.Add(p)
		return nil
	})
	initialHash := commit(t, repo, "initial", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "assets.lock")
		f, _ := wt.Filesystem.Create(p)
		_, _ = f.Write([]byte(initialKilnfileLock))
		_ = f.Close()
		_, _ = wt.Add(p)
		return nil
	})
	middleHash := commit(t, repo, "some other change", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "base.yml")
		f, _ := wt.Filesystem.Create(p)
		_, _ = f.Write([]byte("---\nname: something\n"))
		_ = f.Close()
		_, _ = wt.Add(p)
		return nil
	}, initialHash)
	addKilnfile := commit(t, repo, "some other change", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "Kilnfile")
		f, _ := wt.Filesystem.Create(p)
		buf, _ := yaml.Marshal(cargo.Kilnfile{
			Releases: []cargo.BOSHReleaseTarballSpecification{
				{Name: "banana"},
				{Name: "lemon"},
			},
		})
		_, _ = f.Write(buf)
		_ = f.Close()
		_, _ = wt.Add(p)
		return nil
	}, middleHash)
	badYAML := commit(t, repo, "add some non-yaml", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "Kilnfile.lock")
		f, _ := wt.Filesystem.Create(p)
		_, _ = f.Write([]byte(`{{ if eq tile "ert"}}# this is ERT{{}}\n` + finalKilnfileLock))
		_ = f.Close()
		_, _ = wt.Add(p)

		l := wt.Filesystem.Join(tileDir, "assets.lock")
		_, _ = wt.Remove(l)
		_, _ = wt.Add(l)
		return nil
	}, addKilnfile)
	finalHash := commit(t, repo, "fix bad yaml", func(wt *git.Worktree) error {
		p := wt.Filesystem.Join(tileDir, "Kilnfile.lock")
		f, _ := wt.Filesystem.Create(p)
		_, _ = f.Write([]byte(finalKilnfileLock))
		_ = f.Close()
		_, _ = wt.Add(p)
		return nil
	}, badYAML)
	// END setup

	t.Run("legacy bill of materials", func(t *testing.T) {
		please := NewWithT(t)

		_, kl, err := Kilnfile(repo.Storer, initialHash, "tile")

		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(kl.Releases).To(Equal([]cargo.BOSHReleaseTarballLock{
			{Name: "banana", Version: "0.1.0"},
			{Name: "lemon", Version: "1.1.0"},
		}))
	})

	t.Run("Kilnfile.lock", func(t *testing.T) {
		please := NewWithT(t)

		_, finalKF, err := Kilnfile(repo.Storer, finalHash, "tile/Kilnfile")

		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(finalKF.Releases).To(Equal([]cargo.BOSHReleaseTarballLock{
			{Name: "banana", Version: "0.9.0"},
			{Name: "lemon", Version: "1.9.0"},
			{Name: "apple", Version: "0.0.1"},
		}))
	})

	t.Run("Kilnfile", func(t *testing.T) {
		please := NewWithT(t)

		kf, _, err := Kilnfile(repo.Storer, finalHash, "tile")

		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(kf.Releases).To(Equal([]cargo.BOSHReleaseTarballSpecification{
			{Name: "banana"},
			{Name: "lemon"},
		}))
	})

	t.Run("bad yaml", func(t *testing.T) {
		please := NewWithT(t)

		_, _, err := Kilnfile(repo.Storer, badYAML, "tile")

		please.Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
	})

	t.Run("missing kilnfile", func(t *testing.T) {
		please := NewWithT(t)

		_, _, err := Kilnfile(repo.Storer, initialHashWithoutKilnfile, "tile")

		please.Expect(err).To(MatchError(ContainSubstring("file not found")))
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

func TestWalk(t *testing.T) {
	t.Run("simple walk", func(t *testing.T) {
		// START setup
		tileDir := "tile"
		repo, _ := git.Init(memory.NewStorage(), memfs.New())

		h0 := commit(t, repo, "alpha release", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "version")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("1.0.0-alpha.1\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		})
		h1 := commit(t, repo, "some other change", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "base.yml")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("---\nname: something\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		}, h0)
		hf := commit(t, repo, "ga release", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "version")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("1.0.0\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		}, h1)
		// END setup

		please := NewWithT(t)

		callCount := 0
		err := Walk(repo.Storer, hf, func(*object.Commit) error {
			callCount++
			return nil
		})
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(callCount).To(Equal(3))
	})
	t.Run("walk with invalid commits", func(t *testing.T) {
		// START setup
		repo, _ := git.Init(memory.NewStorage(), memfs.New())

		please := NewWithT(t)

		err := Walk(repo.Storer, plumbing.NewHash("some-invalid-hash"), func(*object.Commit) error {
			return nil
		})
		please.Expect(err).To(HaveOccurred())
	})
	t.Run("walk with input function throwing error", func(t *testing.T) {
		// START setup
		tileDir := "tile"
		repo, _ := git.Init(memory.NewStorage(), memfs.New())

		h0 := commit(t, repo, "alpha release", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "version")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("1.0.0-alpha.1\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		})
		// END setup

		please := NewWithT(t)

		err := Walk(repo.Storer, h0, func(*object.Commit) error {
			return errors.New("some-error")
		})
		please.Expect(err).To(HaveOccurred())
	})
	t.Run("with branch", func(t *testing.T) {
		// START setup
		tileDir := "tile"
		repo, _ := git.Init(memory.NewStorage(), memfs.New())

		h0 := commit(t, repo, "alpha release", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "version")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("1.0.0-alpha.1\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		})
		h1 := commit(t, repo, "some other change", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "base.yml")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("---\nname: something-else\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		}, h0)

		b1 := commit(t, repo, "some other change", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "base.yml")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("---\nname: something\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		}, h0)
		b2 := commit(t, repo, "some other change", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "base.yml")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("---\nname: something-else\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		}, b1)

		hf := commit(t, repo, "ga release", func(wt *git.Worktree) error {
			p := wt.Filesystem.Join(tileDir, "version")
			kf, _ := wt.Filesystem.Create(p)
			_, _ = kf.Write([]byte("1.0.0\n"))
			_ = kf.Close()
			_, _ = wt.Add(p)
			return nil
		}, h1, b2)
		// END setup

		please := NewWithT(t)

		callCount := 0
		err := Walk(repo.Storer, hf, func(*object.Commit) error {
			callCount++
			return nil
		})
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(callCount).To(Equal(5))
	})
}

func commit(t *testing.T, repo *git.Repository, msg string, fn func(wt *git.Worktree) error, parents ...plumbing.Hash) plumbing.Hash {
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
		Author: signature, Committer: signature, Parents: parents,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

/*
---
releases:
- name: banana
- name: lemon
*/
