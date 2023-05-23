package cargo

import (
	"testing"

	. "github.com/onsi/gomega"
)

const (
	someReleaseSourceID = "some-release-source-id"
)

func TestValidate_MissingName(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseSpec{
			{},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseLock{
			{Name: "banana", Version: "1.2.3", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(1))
}

func TestValidate_FloatingRelease(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseSpec{
			{Name: "banana", Version: "1.1.*"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseLock{
			{Name: "banana", Version: "1.1.12", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(0))
}

func TestValidate_MissingLock(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		Releases: []BOSHReleaseSpec{
			{Name: "banana", Version: "1.1.*"},
		},
	}, KilnfileLock{})
	please.Expect(results).To(HaveLen(1))
}

func TestValidate_InvalidConstraint(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseSpec{
			{Name: "banana", Version: "NOT A CONSTRAINT"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseLock{
			{Name: "banana", Version: "1.2.3", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(1))
}

func TestValidate_PinnedRelease(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseSpec{
			{Name: "banana", Version: "1.2.3"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseLock{
			{Name: "banana", Version: "1.2.3", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(0))
}

func TestValidate_release_sources(t *testing.T) {
	t.Run("release source is not found", func(t *testing.T) {
		please := NewWithT(t)
		results := Validate(Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{ID: "ORANGE_SOURCE"},
			},
			Releases: []BOSHReleaseSpec{
				{Name: "lemon"},
				{Name: "orange"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseLock{
				{Name: "lemon", Version: "1.2.3", RemoteSource: "LEMON_SOURCE"},
				{Name: "orange", Version: "1.2.3", RemoteSource: "ORANGE_SOURCE"},
			},
		})
		please.Expect(results).To(HaveLen(1))
		err := results[0]
		please.Expect(err).To(MatchError(And(ContainSubstring("lemon"), ContainSubstring("LEMON_SOURCE"))))
	})
	t.Run("release source is correctly configured", func(t *testing.T) {
		please := NewWithT(t)
		results := Validate(Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{ID: "SOME_TREE"},
			},
			Releases: []BOSHReleaseSpec{
				{Name: "lemon"},
				{Name: "orange"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseLock{
				{Name: "lemon", Version: "1.2.3", RemoteSource: "SOME_TREE"},
				{Name: "orange", Version: "1.2.3", RemoteSource: "SOME_TREE"},
			},
		})
		please.Expect(results).To(BeEmpty())
	})
	t.Run("match on type", func(t *testing.T) {
		please := NewWithT(t)
		results := Validate(Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{Type: ReleaseSourceTypeBOSHIO},
			},
			Releases: []BOSHReleaseSpec{
				{Name: "orange"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseLock{
				{Name: "orange", Version: "1.2.3", RemoteSource: ReleaseSourceTypeBOSHIO},
			},
		})
		please.Expect(results).To(BeEmpty())
	})
	t.Run("do not match on type when id is set", func(t *testing.T) {
		please := NewWithT(t)
		results := Validate(Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{ID: "open source", Type: ReleaseSourceTypeBOSHIO},
			},
			Releases: []BOSHReleaseSpec{
				{Name: "orange"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseLock{
				{Name: "orange", Version: "1.2.3", RemoteSource: ReleaseSourceTypeBOSHIO},
			},
		})
		please.Expect(results).To(HaveLen(1))
	})
	t.Run("github release source", func(t *testing.T) {
		please := NewWithT(t)
		results := Validate(Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{Org: "crhntr", Type: ReleaseSourceTypeGithub},
			},
			Releases: []BOSHReleaseSpec{
				{Name: "hello-tile", GitHubRepository: "https://github.com/crhntr/hello-tile"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseLock{
				{Name: "hello-tile", Version: "1.2.3", RemoteSource: "crhntr"},
			},
		})
		please.Expect(results).To(HaveLen(0))
	})
}

func TestValidate_checkComponentVersionsAndConstraint(t *testing.T) {
	t.Run("no version", func(t *testing.T) {
		please := NewWithT(t)
		r := BOSHReleaseSpec{
			Name: "capi",
		}
		l := BOSHReleaseLock{
			Name:    "capi",
			Version: "2.3.4",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("invalid version constraint", func(t *testing.T) {
		please := NewWithT(t)
		r := BOSHReleaseSpec{
			Name:    "capi",
			Version: "meh",
		}
		l := BOSHReleaseLock{
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
		r := BOSHReleaseSpec{
			Name:    "capi",
			Version: "~2",
		}
		l := BOSHReleaseLock{
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
		r := BOSHReleaseSpec{
			Name:    "capi",
			Version: "~2",
		}
		l := BOSHReleaseLock{
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
