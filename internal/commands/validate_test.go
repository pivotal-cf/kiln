package commands

import (
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	Ω "github.com/onsi/gomega"
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

func TestValidate_validateRelease(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := cargo.ComponentSpec{}
		l := cargo.ComponentLock{}
		err := validateRelease(r, l, 0)
		please.Expect(err).To(Ω.And(
			Ω.HaveOccurred(),
			Ω.MatchError(Ω.ContainSubstring("missing name")),
		))
	})

	t.Run("no version", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := cargo.ComponentSpec{
			Name: "capi",
		}
		l := cargo.ComponentLock{
			Name:    "capi",
			Version: "2.3.4",
		}
		err := validateRelease(r, l, 0)
		please.Expect(err).NotTo(Ω.HaveOccurred())
	})

	t.Run("invalid version constraint", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := cargo.ComponentSpec{
			Name:    "capi",
			Version: "meh",
		}
		l := cargo.ComponentLock{
			Name:    "capi",
			Version: "2.3.4",
		}
		err := validateRelease(r, l, 0)
		please.Expect(err).To(Ω.And(
			Ω.HaveOccurred(),
			Ω.MatchError(Ω.ContainSubstring("invalid version constraint")),
		))
	})

	t.Run("version does not match constraint", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := cargo.ComponentSpec{
			Name:    "capi",
			Version: "~2",
		}
		l := cargo.ComponentLock{
			Name:    "capi",
			Version: "3.0.5",
		}
		err := validateRelease(r, l, 0)
		please.Expect(err).To(Ω.And(
			Ω.HaveOccurred(),
			Ω.MatchError(Ω.ContainSubstring("match constraint")),
		))
	})

	t.Run("invalid lock version", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := cargo.ComponentSpec{
			Name:    "capi",
			Version: "~2",
		}
		l := cargo.ComponentLock{
			Name:    "capi",
			Version: "BAD",
		}
		err := validateRelease(r, l, 0)
		please.Expect(err).To(Ω.And(
			Ω.HaveOccurred(),
			Ω.MatchError(Ω.ContainSubstring("invalid lock version")),
		))
	})
}
