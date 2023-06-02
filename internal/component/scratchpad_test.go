package component

import (
	"net/http"
	"os"
	"testing"

	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: delete this test before merging to main
func TestArtifactoryAgainstTheRealServer(t *testing.T) {
	artifactoryHost := "https://artifactory.eng.vmware.com"
	_, err := http.Get(artifactoryHost)
	if err != nil {
		t.Skip(err)
	}
	source := NewArtifactoryReleaseSource(cargo.ReleaseSourceConfig{
		ArtifactoryHost: artifactoryHost,
		ID:              "shared-releases-ubuntu",
		Repo:            "tanzu-application-services-generic-local",
		Publishable:     true,
		PathTemplate:    "shared-releases/{{.StemcellOS}}/{{.StemcellVersion}}/{{.Name}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz",

		Username: "",
		Password: "",
	})

	t.Run("GetMatchedRelease", func(t *testing.T) {
		t.Parallel()
		lock, err := source.GetMatchedRelease(Spec{
			Name:            "bpm",
			Version:         "1.2.2",
			StemcellVersion: "1.117",
			StemcellOS:      "ubuntu-jammy",
		})
		require.NoError(t, err)
		assert.Equal(t, Lock{
			Name:         "bpm",
			SHA1:         "551a2900eb32b246c8dead5bd5be749c47fce588",
			Version:      "1.2.2",
			RemoteSource: "shared-releases-ubuntu",
			RemotePath:   "shared-releases/ubuntu-jammy/1.117/bpm/bpm-1.2.2-ubuntu-jammy-1.117.tgz",
		}, lock)
	})

	t.Run("FindReleaseVersion", func(t *testing.T) {
		t.Parallel()
		lock, err := source.FindReleaseVersion(Spec{
			Name:            "bpm",
			Version:         "*",
			StemcellVersion: "1.117",
			StemcellOS:      "ubuntu-jammy",
		}, false)
		require.NoError(t, err)
		assert.Equal(t, Lock{
			Name:            "bpm",
			SHA1:            "551a2900eb32b246c8dead5bd5be749c47fce588",
			Version:         "1.2.2",
			StemcellOS:      "ubuntu-jammy",
			StemcellVersion: "1.117",
			RemoteSource:    "shared-releases-ubuntu",
			RemotePath:      "shared-releases/ubuntu-jammy/1.117/bpm/bpm-1.2.2-ubuntu-jammy-1.117.tgz",
		}, lock)
	})

	t.Run("DownloadRelease", func(t *testing.T) {
		t.Parallel()
		dir, err := os.MkdirTemp("", "kiln-DownloadRelease")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(dir, 0744))
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})

		local, err := source.DownloadRelease(dir, Lock{
			Name:         "bpm",
			SHA1:         "551a2900eb32b246c8dead5bd5be749c47fce588",
			Version:      "1.2.2",
			RemoteSource: "shared-releases-ubuntu",
			RemotePath:   "shared-releases/ubuntu-jammy/1.117/bpm/bpm-1.2.2-ubuntu-jammy-1.117.tgz",
		})
		require.NoError(t, err)
		t.Logf("%#v", local)
	})

	t.Error("this should not be on main")
}
