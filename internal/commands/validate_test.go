package commands

import (
	"testing"

	Ω "github.com/onsi/gomega"
	"github.com/go-git/go-billy/v5/osfs"
)

func TestValidate_FloatingRelease(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	validate := Validate{
		FS: osfs.New("testdata/validate/floating-release"),
	}
	err := validate.Execute(nil)
	please.Expect(err).NotTo(Ω.HaveOccurred())
}

func TestValidate_MissingLock(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	validate := Validate{
		FS: osfs.New("testdata/validate/missing-lock"),
	}
	err := validate.Execute(nil)
	please.Expect(err).To(Ω.HaveOccurred())
}

func TestValidate_WrongVersionType(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	validate := Validate{
		FS: osfs.New("testdata/validate/wrong-version-type"),
	}
	err := validate.Execute(nil)
	please.Expect(err).To(Ω.HaveOccurred())
}

func TestValidate_InvalidConstraint(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	validate := Validate{
		FS: osfs.New("testdata/validate/invalid-constraint"),
	}
	err := validate.Execute(nil)
	please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("bpm")))
}

func TestValidate_PinnedRelease(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	validate := Validate{
		FS: osfs.New("testdata/validate/pinned-release"),
	}
	err := validate.Execute(nil)
	please.Expect(err).NotTo(Ω.HaveOccurred())
}
