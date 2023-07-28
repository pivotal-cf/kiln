package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func Test_loadCompileBOSHReleasesParameters(t *testing.T) {
	const lockContents = `{"stemcell_criteria": {"os": "peach", "version": "4.5"}}`

	t.Run("when the directory is empty and default flags are passed", func(t *testing.T) {
		chTempDir(t)

		rd := "releases"
		kf := "Kilnfile"

		_, _, err := loadCompileBOSHReleasesParameters(kf, rd)
		require.Error(t, err)
	})

	t.Run("when the it finds releases and the stemcell", func(t *testing.T) {
		chTempDir(t)

		rd := "releases"
		require.NoError(t, os.MkdirAll(rd, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(rd, "pear.tgz"), []byte("contents"), 0o600))
		kf := "Kilnfile"
		require.NoError(t, os.WriteFile(kf+".lock", []byte(lockContents), 0o600))

		kilnfileLock, releases, err := loadCompileBOSHReleasesParameters(kf, rd)
		require.NoError(t, err)

		assert.Equal(t, cargo.Stemcell{
			OS:      "peach",
			Version: "4.5",
		}, kilnfileLock.Stemcell)

		assert.Equal(t, []string{filepath.Join("releases", "pear.tgz")}, releases)
	})

	t.Run("when no releases exist", func(t *testing.T) {
		chTempDir(t)

		rd := "releases"
		require.NoError(t, os.MkdirAll(rd, 0o700))
		kf := "Kilnfile"
		require.NoError(t, os.WriteFile(kf+".lock", []byte(lockContents), 0o600))

		_, _, err := loadCompileBOSHReleasesParameters(kf, rd)
		require.ErrorContains(t, err, "no BOSH release tarballs found")
	})

	t.Run("when no Kilnfile lock exists", func(t *testing.T) {
		chTempDir(t)

		rd := "releases"
		require.NoError(t, os.MkdirAll(rd, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(rd, "pear.tgz"), []byte("contents"), 0o600))
		kf := "Kilnfile"
		require.NoError(t, os.WriteFile(kf, []byte(lockContents), 0o600))

		_, _, err := loadCompileBOSHReleasesParameters(kf, rd)
		require.ErrorContains(t, err, "failed to read Kilnfile.lock")
	})
}

func chTempDir(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restoring working directory: %v", err)
		}
	})
}
