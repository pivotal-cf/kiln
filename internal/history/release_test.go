package history_test

import (
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pivotal-cf/kiln/internal/component"
	"os"
	"regexp"
	"testing"

	Ω "github.com/onsi/gomega"

	"github.com/go-git/go-git/v5"

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

	_, err = history.TileVersionFileBoshReleaseList(repo, regexp.MustCompile(`^rel/2\.\d+$`), []string{"garden-runc"},
		history.StopAfter(10000),
		history.FindBoshRelease(component.Spec{Name: "garden-runc", Version: "1.19.29"}),
	)

	please.Expect(err).NotTo(Ω.HaveOccurred())
}

func TestVersionFileBoshReleaseList(t *testing.T) {
	please := Ω.NewWithT(t)

	repo, err := git.PlainOpen(checkIfLocalTasRepo(t))
	please.Expect(err).NotTo(Ω.HaveOccurred())

	records, err := history.TileReleaseBoshReleaseList(repo, "garden-runc", "cflinuxfs3")

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(records).To(Ω.ContainElement(history.ReleaseMapping{
		Tile: component.Spec{Name: "tas", Version: "2.11.4"},
		Bosh: component.Spec{Name: "garden-runc", Version: "1.19.29"},
	}))
	please.Expect(records).To(Ω.ContainElement(history.ReleaseMapping{
		Tile: component.Spec{Name: "ist", Version: "2.11.3"},
		Bosh: component.Spec{Name: "garden-runc", Version: "1.19.28"},
	}))
	please.Expect(records).To(Ω.ContainElement(history.ReleaseMapping{
		Tile: component.Spec{Name: "tasw", Version: "2.7.29"},
		Bosh: component.Spec{Name: "garden-runc", Version: "1.19.25"},
	}))
	please.Expect(records).To(Ω.ContainElement(history.ReleaseMapping{
		Tile: component.Spec{Name: "ist", Version: "2.9.20"},
		Bosh: component.Spec{Name: "cflinuxfs3", Version: "0.238.0"},
	}))
}

func TestFindBoshTileRelease(t *testing.T) {
	result := history.FindBoshTileRelease(component.Spec{
		Name:    "product",
		Version: "1.2,3",
	})(0, object.Commit{}, []history.ReleaseMapping{
		{
			Tile: component.Spec{
				Name:    "product",
				Version: "1.2,3",
			},
			Bosh: component.Spec{
				Name:    "release",
				Version: "1.2,3",
			},
		},
	})
	if !result {
		t.Fail()
	}
}
