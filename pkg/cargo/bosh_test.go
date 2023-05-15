package cargo_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleasesMetadataFromReleasesDirectory(t *testing.T) {
	releaseTarballPath, found := os.LookupEnv("RELEASE_TARBALL")
	if !found {
		releaseTarballPath = filepath.Join("testdata", "bosh", "releases", "bpm-release-1.2.1.tgz")
	}
	boshReleaseMetadata, err := cargo.BOSHReleaseTileMetadataFromGzippedTarball(os.DirFS(filepath.Dir(releaseTarballPath)), filepath.Base(releaseTarballPath))
	require.NoError(t, err)
	assert.Equal(t, "bpm", boshReleaseMetadata.Name)
	assert.Equal(t, "1.2.1", boshReleaseMetadata.Version)
}

func TestReleasesMetadataFromReleasesDirectory_errors(t *testing.T) {
	t.Run("release does not exist", func(t *testing.T) {
		dir := fstest.MapFS{}
		_, err := cargo.BOSHReleaseTileMetadataFromGzippedTarball(dir, "banana.tgz")
		require.Error(t, err)
	})
	t.Run("wrong file type", func(t *testing.T) {
		r := bytes.NewBuffer(nil)
		tarball := tar.NewWriter(r)
		_ = tarball.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "release.MF"})
		_, _ = tarball.Write([]byte("{}"))
		_ = tarball.Close()

		dir := fstest.MapFS{
			"release.tar": &fstest.MapFile{
				Data: r.Bytes(),
			},
		}
		_, err := cargo.BOSHReleaseTileMetadataFromGzippedTarball(dir, "release.tar")
		require.Error(t, err)
	})
}

func TestBOSHReleaseManifestAndLicense(t *testing.T) {
	t.Run("missing release", func(t *testing.T) {
		dir := fstest.MapFS{}
		_, _, err := cargo.BOSHReleaseManifestAndLicense(dir, "banana")
		require.Error(t, err)
	})
	t.Run("wrong file type", func(t *testing.T) {
		r := bytes.NewBuffer(nil)
		tarball := tar.NewWriter(r)
		_ = tarball.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "release.MF"})
		_, _ = tarball.Write([]byte("{}"))
		_ = tarball.Close()

		dir := fstest.MapFS{
			"release.tar": &fstest.MapFile{
				Data: r.Bytes(),
			},
		}
		_, _, err := cargo.BOSHReleaseManifestAndLicense(dir, "release.tar")
		require.Error(t, err)
	})
}
