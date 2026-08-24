//go:build !windows

package carvel

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCanonicalizeBoshReleasePreservesFileModeUnderRestrictiveUmask(t *testing.T) {
	// os.WriteFile's create mode is masked by the umask, os.Chmod is not, so
	// use bits umask 022 would strip to prove no silent narrowing on write.
	// syscall.Umask is Unix-only, hence the build tag.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")
	buildSyntheticBoshRelease(t, path, 0)

	wantMode := os.FileMode(0o666)
	if err := os.Chmod(path, wantMode); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	oldUmask := syscall.Umask(0o022)
	defer syscall.Umask(oldUmask)

	if _, err := canonicalizeBoshRelease(path, "0.1.1"); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != wantMode {
		t.Errorf("file mode = %o, want %o (umask should not narrow the preserved mode)", info.Mode().Perm(), wantMode)
	}
}
