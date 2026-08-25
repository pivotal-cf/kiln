package helper

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
)

// BillyOS is a billy.Filesystem backed directly by the os package, with no
// chroot-style boundary enforcement: paths are passed straight through to
// the standard library, so both relative (resolved against the process's
// working directory) and absolute paths work exactly like plain os.Open,
// os.Stat, etc.
//
// go-billy's osfs.New("") used to behave this way too, but as of go-billy
// v5.9.0 (the path-traversal/symlink-loop CVE fix), its Open/Create/Stat/
// OpenFile/ReadDir now resolve every path relative to the chroot root before
// checking it, which fails with billy.ErrCrossedBoundary for any absolute
// path when the root is "" (see followedPath/relativeToRoot in
// go-billy/v5/helper/chroot). Kiln uses osfs.New("") in several places
// specifically to mean "no containment, just talk to the real filesystem" so
// BillyOS exists to provide that without going through billy's boundary
// checks at all.
type BillyOS struct{}

// NewBillyOS returns a billy.Filesystem with no path containment, backed
// directly by the os package.
func NewBillyOS() BillyOS { return BillyOS{} }

func (BillyOS) Create(filename string) (billy.File, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return billyOSFile{f}, nil
}

func (BillyOS) Open(filename string) (billy.File, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return billyOSFile{f}, nil
}

func (BillyOS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	f, err := os.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}
	return billyOSFile{f}, nil
}

func (BillyOS) Stat(filename string) (os.FileInfo, error) { return os.Stat(filename) }

func (BillyOS) Rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

func (BillyOS) Remove(filename string) error { return os.Remove(filename) }

func (BillyOS) Join(elem ...string) string { return filepath.Join(elem...) }

func (BillyOS) ReadDir(path string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	infos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (BillyOS) MkdirAll(filename string, perm os.FileMode) error {
	return os.MkdirAll(filename, perm)
}

func (BillyOS) TempFile(dir, prefix string) (billy.File, error) {
	f, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return nil, err
	}
	return billyOSFile{f}, nil
}

func (BillyOS) Lstat(filename string) (os.FileInfo, error) { return os.Lstat(filename) }

func (BillyOS) Symlink(target, link string) error { return os.Symlink(target, link) }

func (BillyOS) Readlink(link string) (string, error) { return os.Readlink(link) }

// Chroot returns a real, boundary-enforcing filesystem rooted at path: unlike
// BillyOS itself, a non-empty root is exactly what go-billy's chroot
// machinery is designed for, so this delegates to it.
func (BillyOS) Chroot(path string) (billy.Filesystem, error) { return osfs.New(path), nil }

func (BillyOS) Root() string { return "" }

// billyOSFile adapts *os.File to billy.File, which additionally requires
// Lock/Unlock. Kiln is a single-process CLI and never relies on advisory
// file locking, so these are no-ops.
type billyOSFile struct{ *os.File }

func (billyOSFile) Lock() error   { return nil }
func (billyOSFile) Unlock() error { return nil }

var _ billy.Filesystem = BillyOS{}
