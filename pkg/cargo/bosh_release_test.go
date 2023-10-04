package cargo_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pivotal-cf/kiln/pkg/cargo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal-cf/kiln/pkg/proofing"

	. "github.com/onsi/gomega"
)

func TestReadReleaseFromFile(t *testing.T) {
	please := NewWithT(t)

	buf := bytes.NewBuffer(nil)
	releaseMetadata, err := cargo.ReadBOSHReleaseFromFile(filepath.Join("testdata", "tile-0.1.2.pivotal"), "hello-release", "v0.1.4", buf)
	please.Expect(err).NotTo(HaveOccurred())

	please.Expect(releaseMetadata).To(Equal(proofing.Release{
		File:    "hello-release-v0.1.4-ubuntu-xenial-621.256.tgz",
		Name:    "hello-release",
		SHA1:    "c471ac6371eb8fc24508b14d9a49a44f9a5ef98c",
		Version: "v0.1.4",
	}))

	_, err = io.ReadAll(buf)
	please.Expect(err).NotTo(HaveOccurred())
}

func TestReadBOSHReleaseManifestsFromTarballs(t *testing.T) {
	boshReleases, err := cargo.OpenBOSHReleaseManifestsFromTarballs(filepath.Join("testdata", "bpm-1.1.21-ubuntu-xenial-621.463.tgz"), filepath.Join("testdata", "bpm-1.1.21.tgz"))
	require.NoError(t, err)
	require.Len(t, boshReleases, 2)
	assert.Equal(t, "be5b1710f33128f6c864eae1d97effddb94dd3ac", boshReleases[0].SHA1)
	assert.Equal(t, "519b78f2f3333a7b9c000bbef325e12a2f36996d", boshReleases[1].SHA1)
	assert.Equal(t, filepath.Join("testdata", "bpm-1.1.21-ubuntu-xenial-621.463.tgz"), boshReleases[0].FilePath)
	assert.Equal(t, filepath.Join("testdata", "bpm-1.1.21.tgz"), boshReleases[1].FilePath)
}

func TestReadProductTemplatePartFromBOSHReleaseTarball(t *testing.T) {
	t.Run("when the release is compiled", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "bpm-1.1.21-ubuntu-xenial-621.463.tgz"))
		require.NoError(t, err)
		t.Cleanup(func() {
			closeAndIgnoreError(f)
		})

		result, err := cargo.ReadProductTemplatePartFromBOSHReleaseTarball(f)
		require.NoError(t, err)

		require.Equal(t, cargo.BOSHReleaseManifest{
			Name:       "bpm",
			Version:    "1.1.21",
			CommitHash: "fd88358", UncommittedChanges: false, CompiledPackages: []cargo.CompiledBOSHReleasePackage{
				{
					Name:         "bpm",
					Version:      "be375c78c703cea04667ea7cbbc6d024bb391182",
					Fingerprint:  "be375c78c703cea04667ea7cbbc6d024bb391182",
					SHA1:         "b67ab0ceb0cab69a170521bb1a77c91a8546ac21",
					Dependencies: []string{"golang-1-linux", "bpm-runc", "tini"},
					Stemcell:     "ubuntu-xenial/621.463",
				},
				{
					Name:         "test-server",
					Version:      "12eba471a2c3dddb8547ef03c23a3231d1f62e6c",
					Fingerprint:  "12eba471a2c3dddb8547ef03c23a3231d1f62e6c",
					SHA1:         "7ab0c2066c63eb5c5dd2c06d35b73376f4ad9a81",
					Dependencies: []string{"golang-1-linux"},
					Stemcell:     "ubuntu-xenial/621.463",
				},
				{
					Name:         "bpm-runc",
					Version:      "464c6e6611f814bd12016156bf3e682486f34672",
					Fingerprint:  "464c6e6611f814bd12016156bf3e682486f34672",
					SHA1:         "bacd602ee0830a30c17b7a502aa0021a6739a3ff",
					Dependencies: []string{"golang-1-linux"},
					Stemcell:     "ubuntu-xenial/621.463",
				},
				{
					Name:         "golang-1-linux",
					Version:      "2336380dbf01a44020813425f92be34685ce118bf4767406c461771cfef14fc9",
					Fingerprint:  "2336380dbf01a44020813425f92be34685ce118bf4767406c461771cfef14fc9",
					SHA1:         "e8dad3e51eeb5f5fb41dd56bbb8a3ec9655bd4f7",
					Dependencies: []string{},
					Stemcell:     "ubuntu-xenial/621.463",
				},
				{
					Name:         "tini",
					Version:      "3d7b02f3eeb480b9581bec4a0096dab9ebdfa4bc",
					Fingerprint:  "3d7b02f3eeb480b9581bec4a0096dab9ebdfa4bc",
					SHA1:         "347c76d509ad4b82d99bbbb4b291768ff90b0fba",
					Dependencies: []string{},
					Stemcell:     "ubuntu-xenial/621.463",
				},
			},
		}, result)
	})

	t.Run("when the release is not compiled", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "bpm-1.1.21.tgz"))
		require.NoError(t, err)
		t.Cleanup(func() {
			closeAndIgnoreError(f)
		})

		result, err := cargo.ReadProductTemplatePartFromBOSHReleaseTarball(f)
		require.NoError(t, err)

		require.Equal(t, cargo.BOSHReleaseManifest{
			Name:               "bpm",
			Version:            "1.1.21",
			CommitHash:         "fd88358",
			UncommittedChanges: false,
			Packages: []cargo.BOSHReleasePackage{
				{
					Name:         "bpm",
					Version:      "be375c78c703cea04667ea7cbbc6d024bb391182",
					Fingerprint:  "be375c78c703cea04667ea7cbbc6d024bb391182",
					SHA1:         "6ae70da9768bd7333b883463e089c65bea44c685",
					Dependencies: []string{"golang-1-linux", "bpm-runc", "tini"},
				},
				{
					Name:         "bpm-runc",
					Version:      "464c6e6611f814bd12016156bf3e682486f34672",
					Fingerprint:  "464c6e6611f814bd12016156bf3e682486f34672",
					SHA1:         "1a141265caca8b61209349efdbd4ecbc3f802526",
					Dependencies: []string{"golang-1-linux"},
				},
				{
					Name:         "golang-1-linux",
					Version:      "2336380dbf01a44020813425f92be34685ce118bf4767406c461771cfef14fc9",
					Fingerprint:  "2336380dbf01a44020813425f92be34685ce118bf4767406c461771cfef14fc9",
					SHA1:         "sha256:c440fd7aa6d179a6891f765c46f69fa2b6cbadd8da3b019a4fb27fc9001a4f1f",
					Dependencies: []string{},
				},
				{
					Name:         "test-server",
					Version:      "12eba471a2c3dddb8547ef03c23a3231d1f62e6c",
					Fingerprint:  "12eba471a2c3dddb8547ef03c23a3231d1f62e6c",
					SHA1:         "28e74c692da08a1c065052d118b6c1324caa6e8b",
					Dependencies: []string{"golang-1-linux"},
				},
				{
					Name:         "tini",
					Version:      "3d7b02f3eeb480b9581bec4a0096dab9ebdfa4bc",
					Fingerprint:  "3d7b02f3eeb480b9581bec4a0096dab9ebdfa4bc",
					SHA1:         "3d16adbc5ed9bc46a46503cd4f12d883a28fa991",
					Dependencies: []string{},
				},
			},
		}, result)
	})
}

func TestOpenBOSHReleaseTarball(t *testing.T) {
	t.Run("release tarball does not exist", func(t *testing.T) {
		temporaryTestDirectory := t.TempDir()

		releaseFilepath := filepath.Join(temporaryTestDirectory, "release.tgz")

		_, err := cargo.OpenBOSHReleaseTarball(releaseFilepath)

		require.Error(t, err)
	})

	t.Run("release tarball is empty not exist", func(t *testing.T) {
		temporaryTestDirectory := t.TempDir()

		releaseFilepath := filepath.Join(temporaryTestDirectory, "release.tgz")
		{
			f, err := os.Create(releaseFilepath)
			require.NoError(t, err)
			require.NoError(t, f.Close())
		}

		_, err := cargo.OpenBOSHReleaseTarball(releaseFilepath)
		require.EqualError(t, err, "BOSH release tarball release.tgz is an empty file")
	})
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}
