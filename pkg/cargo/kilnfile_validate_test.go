package cargo

import (
	"testing"

	Ω "github.com/onsi/gomega"
)

func TestValidate_MissingName(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{},
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3"},
		},
	})
	please.Expect(results).To(Ω.HaveLen(1))
}

func TestValidate_FloatingRelease(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.1.*"},
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.1.12"},
		},
	})
	please.Expect(results).To(Ω.HaveLen(0))
}

func TestValidate_MissingLock(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.1.*"},
		},
	}, KilnfileLock{})
	please.Expect(results).To(Ω.HaveLen(1))
}

func TestValidate_InvalidConstraint(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "NOT A CONSTRAINT"},
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3"},
		},
	})
	please.Expect(results).To(Ω.HaveLen(1))
}

func TestValidate_PinnedRelease(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.2.3"},
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3"},
		},
	})
	please.Expect(results).To(Ω.HaveLen(0))
}

func TestValidate_checkComponentVersionsAndConstraint(t *testing.T) {
	t.Run("no version", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := ComponentSpec{
			Name: "capi",
		}
		l := ComponentLock{
			Name:    "capi",
			Version: "2.3.4",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).NotTo(Ω.HaveOccurred())
	})

	t.Run("invalid version constraint", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := ComponentSpec{
			Name:    "capi",
			Version: "meh",
		}
		l := ComponentLock{
			Name:    "capi",
			Version: "2.3.4",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).To(Ω.And(
			Ω.HaveOccurred(),
			Ω.MatchError(Ω.ContainSubstring("invalid version constraint")),
		))
	})

	t.Run("version does not match constraint", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := ComponentSpec{
			Name:    "capi",
			Version: "~2",
		}
		l := ComponentLock{
			Name:    "capi",
			Version: "3.0.5",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).To(Ω.And(
			Ω.HaveOccurred(),
			Ω.MatchError(Ω.ContainSubstring("match constraint")),
		))
	})

	t.Run("invalid lock version", func(t *testing.T) {
		please := Ω.NewWithT(t)
		r := ComponentSpec{
			Name:    "capi",
			Version: "~2",
		}
		l := ComponentLock{
			Name:    "capi",
			Version: "BAD",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).To(Ω.And(
			Ω.HaveOccurred(),
			Ω.MatchError(Ω.ContainSubstring("invalid lock version")),
		))
	})
}

func Test_checkStemcell_valid_kilfiles(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := checkStemcell(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.2.3"},
			{Name: "lemon", Version: "2.2.2"},
		},
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "500.*",
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3", StemcellOS: "fruit", StemcellVersion: "500.4"},
			{Name: "lemon", Version: "2.2.2"},
		},
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "500.4",
		},
	})
	please.Expect(results).To(Ω.HaveLen(0))
}

func Test_checkStemcell_wrong_version(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := checkStemcell(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.2.3"},
			{Name: "lemon", Version: "2.2.2"},
		},
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "500.*",
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3", StemcellOS: "fruit", StemcellVersion: "400"},
			{Name: "lemon", Version: "2.2.2"},
		},
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "500.4",
		},
	})
	please.Expect(results).To(Ω.HaveLen(1))
	please.Expect(results[0]).To(Ω.MatchError(Ω.ContainSubstring("has stemcell version that does not match the stemcell lock")))
}

func Test_checkStemcell_wrong_os_name(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := checkStemcell(Kilnfile{
		Releases: []ComponentSpec{
			{Name: "banana", Version: "1.2.3"},
			{Name: "lemon", Version: "2.2.2"},
		},
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "500.*",
		},
	}, KilnfileLock{
		Releases: []ComponentLock{
			{Name: "banana", Version: "1.2.3", StemcellOS: "soap", StemcellVersion: "500.4"},
			{Name: "lemon", Version: "2.2.2"},
		},
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "500.4",
		},
	})
	please.Expect(results).To(Ω.HaveLen(1))
	please.Expect(results[0]).To(Ω.MatchError(Ω.ContainSubstring("stemcell os that does not match the stemcell lock os")))
}

func Test_checkStemcell_invalid_version_lock(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := checkStemcell(Kilnfile{
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "500.0",
		},
	}, KilnfileLock{
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "FAIL",
		},
	})
	please.Expect(results).To(Ω.HaveLen(1))
	please.Expect(results[0]).To(Ω.MatchError(Ω.ContainSubstring("invalid lock version")))
}

func Test_checkStemcell_invalid_version_constraint(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := checkStemcell(Kilnfile{
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "FAIL",
		},
	}, KilnfileLock{
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "2.0.0",
		},
	})
	please.Expect(results).To(Ω.HaveLen(1))
	please.Expect(results[0]).To(Ω.MatchError(Ω.ContainSubstring("invalid version constraint")))
}

func Test_checkStemcell_lock_version_does_not_match_constraint(t *testing.T) {
	t.Parallel()
	please := Ω.NewWithT(t)
	results := checkStemcell(Kilnfile{
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "400.*",
		},
	}, KilnfileLock{
		Stemcell: Stemcell{
			OS:      "fruit",
			Version: "111.222",
		},
	})
	please.Expect(results).To(Ω.HaveLen(1))
	please.Expect(results[0]).To(Ω.MatchError(Ω.ContainSubstring("does not match constraint")))
}
