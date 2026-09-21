package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestResolveReleasesDirectory_DefaultAnchorsToSourcePath(t *testing.T) {
	sourcePath := filepath.Join(string(os.PathSeparator), "tile", "source")

	require.Equal(t, filepath.Join(sourcePath, "releases"), resolveReleasesDirectory("", sourcePath),
		"an empty value (flag unset, or carvel rebake which has no --releases-directory flag) must anchor to sourcePath, not be left CWD-relative")
}

func TestResolveReleasesDirectory_ExplicitOverridePassesThrough(t *testing.T) {
	sourcePath := filepath.Join(string(os.PathSeparator), "tile", "source")

	relOverride := filepath.Join("custom", "releases")
	wantRel, err := filepath.Abs(relOverride)
	require.NoError(t, err)
	require.Equal(t, wantRel, resolveReleasesDirectory(relOverride, sourcePath),
		"an explicit override must not be anchored to sourcePath, matching current behavior for any caller that sets --releases-directory")

	absOverride := filepath.Join(string(os.PathSeparator), "absolute", "releases")
	require.Equal(t, absOverride, resolveReleasesDirectory(absOverride, sourcePath),
		"an explicit absolute override must be returned unchanged")

	require.Equal(t, mustAbs(t, "releases"), resolveReleasesDirectory("releases", sourcePath),
		"an explicit --releases-directory releases is no longer conflated with the unset default; it resolves like any other relative override")
}

func mustAbs(t *testing.T, p string) string {
	abs, err := filepath.Abs(p)
	require.NoError(t, err)
	return abs
}

func TestWriteStandardKilnfileLock_PreservesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	lockfilePath := filepath.Join(tmpDir, "Kilnfile.lock")

	initialLock := cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
			{Name: "cf-cli", Version: "1.0.0"},
			{Name: "smoke-tests", Version: "2.0.0"},
		},
	}
	data, err := yaml.Marshal(&initialLock)
	require.NoError(t, err)
	err = os.WriteFile(lockfilePath, data, 0644)
	require.NoError(t, err)

	err = writeStandardKilnfileLock(lockfilePath, "ear-k8s-runtime", "3.0.0", "path/to/remote", "my-source", "fake-sha1")
	require.NoError(t, err)

	data, err = os.ReadFile(lockfilePath)
	require.NoError(t, err)

	var updatedLock cargo.KilnfileLock
	err = yaml.Unmarshal(data, &updatedLock)
	require.NoError(t, err)

	require.Len(t, updatedLock.Releases, 3)
	require.Equal(t, "cf-cli", updatedLock.Releases[0].Name)
	require.Equal(t, "smoke-tests", updatedLock.Releases[1].Name)
	require.Equal(t, "ear-k8s-runtime", updatedLock.Releases[2].Name)
	require.Equal(t, "3.0.0", updatedLock.Releases[2].Version)
	require.Equal(t, "path/to/remote", updatedLock.Releases[2].RemotePath)
	require.Equal(t, "my-source", updatedLock.Releases[2].RemoteSource)
	require.Equal(t, "fake-sha1", updatedLock.Releases[2].SHA1)
}

func TestWriteStandardKilnfileLock_FailsRatherThanOverwriteUnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root bypasses file permission checks")
	}

	tmpDir := t.TempDir()
	lockfilePath := filepath.Join(tmpDir, "Kilnfile.lock")

	initialLock := cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
			{Name: "cf-cli", Version: "1.0.0"},
		},
	}
	data, err := yaml.Marshal(&initialLock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(lockfilePath, data, 0644))
	require.NoError(t, os.Chmod(lockfilePath, 0000))
	defer func() { _ = os.Chmod(lockfilePath, 0644) }()

	err = writeStandardKilnfileLock(lockfilePath, "ear-k8s-runtime", "3.0.0", "path/to/remote", "my-source", "fake-sha1")
	require.Error(t, err, "an unreadable (not missing) lockfile must not be silently treated as absent")

	require.NoError(t, os.Chmod(lockfilePath, 0644))
	onDisk, err := os.ReadFile(lockfilePath)
	require.NoError(t, err)
	require.Equal(t, data, onDisk)
}
