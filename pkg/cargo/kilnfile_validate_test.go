package cargo

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidate_MissingName(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{},
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3"},
		},
	})
	please.Expect(results).To(HaveLen(1))
}

func TestValidate_FloatingRelease(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.1.*"},
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.1.12"},
		},
	})
	please.Expect(results).To(HaveLen(0))
}

func TestValidate_MissingLock(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.1.*"},
		},
	}, KilnfileLock{})
	please.Expect(results).To(HaveLen(1))
}

func TestValidate_InvalidConstraint(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "NOT A CONSTRAINT"},
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3"},
		},
	})
	please.Expect(results).To(HaveLen(1))
}

func TestValidate_PinnedRelease(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.2.3"},
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3"},
		},
	})
	please.Expect(results).To(HaveLen(0))
}

func TestValidate_checkComponentVersionsAndConstraint(t *testing.T) {
	t.Run("no version", func(t *testing.T) {
		please := NewWithT(t)
		r := ComponentSpec{
			Name: "capi",
		}
		l := ComponentLock{
			Name:    "capi",
			Version: "2.3.4",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("invalid version constraint", func(t *testing.T) {
		please := NewWithT(t)
		r := ComponentSpec{
			Name:    "capi",
			Version: "meh",
		}
		l := ComponentLock{
			Name:    "capi",
			Version: "2.3.4",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).To(And(
			HaveOccurred(),
			MatchError(ContainSubstring("invalid version constraint")),
		))
	})

	t.Run("version does not match constraint", func(t *testing.T) {
		please := NewWithT(t)
		r := ComponentSpec{
			Name:    "capi",
			Version: "~2",
		}
		l := ComponentLock{
			Name:    "capi",
			Version: "3.0.5",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).To(And(
			HaveOccurred(),
			MatchError(ContainSubstring("match constraint")),
		))
	})

	t.Run("invalid lock version", func(t *testing.T) {
		please := NewWithT(t)
		r := ComponentSpec{
			Name:    "capi",
			Version: "~2",
		}
		l := ComponentLock{
			Name:    "capi",
			Version: "BAD",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).To(And(
			HaveOccurred(),
			MatchError(ContainSubstring("invalid lock version")),
		))
	})
}
