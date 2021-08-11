package history_test

import (
	"os"
	"regexp"
	"testing"

	Ω "github.com/onsi/gomega"

	"github.com/go-git/go-git/v5"

	"github.com/pivotal-cf/kiln/pkg/release"
	"github.com/pivotal-cf/kiln/internal/history"
)

func checkIfLocalTasRepo(t *testing.T) string {
	t.Helper()
	p := os.Getenv("TAS_REPO_PATH")
	if p == "" {
		p = "../../../tas"
	}
	if _, err := os.Stat(p); err != nil {
		t.Logf("skipping: no TAS repo: %s", err)
		t.SkipNow()
	}
	return p
}

func TestTileVersionFileBoshReleaseList(t *testing.T) {
	please := Ω.NewWithT(t)

	repo, err := git.PlainOpen(checkIfLocalTasRepo(t))
	please.Expect(err).NotTo(Ω.HaveOccurred())

	result, err := history.TileVersionFileBoshReleaseList(repo, regexp.MustCompile(`^rel/2\.\d+$`), []string{"garden-runc"},
		history.StopAfter(10000),
		history.FindBoshRelease(release.ID{Name: "garden-runc", Version: "1.19.29"}),
	)

	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(result).To(Ω.HaveLen(15))
}

func TestVersionFileBoshReleaseList(t *testing.T) {
	please := Ω.NewWithT(t)

	repo, err := git.PlainOpen(checkIfLocalTasRepo(t))
	please.Expect(err).NotTo(Ω.HaveOccurred())

	records, err := history.TileReleaseBoshReleaseList(repo, "garden-runc", "cflinuxfs3")

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(records).To(Ω.ContainElement(history.ReleaseMapping{
		Tile: release.ID{Name: "tas", Version: "2.11.4"},
		Bosh: release.ID{Name: "garden-runc", Version: "1.19.29"},
	}))
	please.Expect(records).To(Ω.ContainElement(history.ReleaseMapping{
		Tile: release.ID{Name: "ist", Version: "2.11.3"},
		Bosh: release.ID{Name: "garden-runc", Version: "1.19.28"},
	}))
	please.Expect(records).To(Ω.ContainElement(history.ReleaseMapping{
		Tile: release.ID{Name: "tasw", Version: "2.7.29"},
		Bosh: release.ID{Name: "garden-runc", Version: "1.19.25"},
	}))
	please.Expect(records).To(Ω.ContainElement(history.ReleaseMapping{
		Tile: release.ID{Name: "ist", Version: "2.9.20"},
		Bosh: release.ID{Name: "cflinuxfs3", Version: "0.238.0"},
	}))
}
