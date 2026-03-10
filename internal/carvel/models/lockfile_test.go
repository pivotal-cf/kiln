package models_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/carvel/models"
)

func TestCarvelLockfileRoundTrip(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "Kilnfile.lock")

	original := models.CarvelLockfile{
		Release: models.CarvelReleaseLock{
			Name:       "my-tile",
			Version:    "1.2.3",
			RemotePath: "bosh-releases/my-tile/my-tile-1.2.3.tgz",
			SHA256:     "abc123def456",
		},
	}

	err := original.WriteFile(lockPath)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(lockPath).To(BeAnExistingFile())

	loaded, err := models.ReadCarvelLockfile(lockPath)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(loaded).To(Equal(original))
}

func TestReadCarvelLockfileNotFound(t *testing.T) {
	g := NewWithT(t)

	_, err := models.ReadCarvelLockfile("/nonexistent/path/Kilnfile.lock")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to read lockfile"))
}

func TestReadCarvelLockfileInvalidYAML(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "Kilnfile.lock")
	err := os.WriteFile(lockPath, []byte("release:\n  name: [unterminated"), 0644)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = models.ReadCarvelLockfile(lockPath)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to parse lockfile"))
}
