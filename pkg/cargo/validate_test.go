package cargo

import (
	"testing"

	. "github.com/onsi/gomega"
)

const (
	someReleaseSourceID = "some-release-source-id"
)

func TestValidate_MissingNameInSpec(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseTarballSpecification{
			{Name: ""},
			{Name: "banana"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseTarballLock{
			{Name: "apple", Version: "1.2.3", RemoteSource: someReleaseSourceID},
			{Name: "banana", Version: "1.2.3", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(2))
}

func TestValidate_MissingNameInLock(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseTarballSpecification{
			{Name: "apple"},
			{Name: "banana"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseTarballLock{
			{Name: "", Version: "1.2.3", RemoteSource: someReleaseSourceID},
			{Name: "banana", Version: "1.2.3", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(2))
}

func TestValidate_FloatingRelease(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseTarballSpecification{
			{Name: "banana", Version: "1.1.*"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseTarballLock{
			{Name: "banana", Version: "1.1.12", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(0))
}

func TestValidate_MissingSpec(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseTarballSpecification{
			{Name: "banana"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseTarballLock{
			{Name: "apple", Version: "1.2.3", RemoteSource: someReleaseSourceID},
			{Name: "banana", Version: "1.2.3", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(1))
}

func TestValidate_MissingLock(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseTarballSpecification{
			{Name: "banana", Version: "1.1.*"},
			{Name: "apple", Version: "1.1.*"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseTarballLock{
			{Name: "banana", Version: "1.1.3", RemoteSource: someReleaseSourceID},
		},
	})
	please.Expect(results).To(HaveLen(1))
}

func TestValidate_InvalidConstraint(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
	results := Validate(Kilnfile{
		ReleaseSources: []ReleaseSourceConfig{
			{ID: someReleaseSourceID},
		},
		Releases: []BOSHReleaseTarballSpecification{
			{Name: "banana", Version: "NOT A CONSTRAINT"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseTarballLock{
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
		Releases: []BOSHReleaseTarballSpecification{
			{Name: "banana", Version: "1.2.3"},
		},
	}, KilnfileLock{
		Releases: []BOSHReleaseTarballLock{
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
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "lemon"},
				{Name: "orange"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
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
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "lemon"},
				{Name: "orange"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
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
				{Type: BOSHReleaseTarballSourceTypeBOSHIO},
			},
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "orange"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
				{Name: "orange", Version: "1.2.3", RemoteSource: BOSHReleaseTarballSourceTypeBOSHIO},
			},
		})
		please.Expect(results).To(BeEmpty())
	})
	t.Run("do not match on type when id is set", func(t *testing.T) {
		please := NewWithT(t)
		results := Validate(Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{ID: "open source", Type: BOSHReleaseTarballSourceTypeBOSHIO},
			},
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "orange"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
				{Name: "orange", Version: "1.2.3", RemoteSource: BOSHReleaseTarballSourceTypeBOSHIO},
			},
		})
		please.Expect(results).To(HaveLen(1))
	})
	t.Run("github release source", func(t *testing.T) {
		please := NewWithT(t)
		results := Validate(Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{Org: "crhntr", Type: BOSHReleaseTarballSourceTypeGithub},
			},
			Releases: []BOSHReleaseTarballSpecification{
				{Name: "hello-tile", GitHubRepository: "https://github.com/crhntr/hello-tile"},
			},
		}, KilnfileLock{
			Releases: []BOSHReleaseTarballLock{
				{Name: "hello-tile", Version: "1.2.3", RemoteSource: "crhntr"},
			},
		})
		please.Expect(results).To(HaveLen(0))
	})
}

func TestValidate_checkComponentVersionsAndConstraint(t *testing.T) {
	t.Run("no version", func(t *testing.T) {
		please := NewWithT(t)
		r := BOSHReleaseTarballSpecification{
			Name: "capi",
		}
		l := BOSHReleaseTarballLock{
			Name:    "capi",
			Version: "2.3.4",
		}
		err := checkComponentVersionsAndConstraint(r, l, 0)
		please.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("invalid version constraint", func(t *testing.T) {
		please := NewWithT(t)
		r := BOSHReleaseTarballSpecification{
			Name:    "capi",
			Version: "meh",
		}
		l := BOSHReleaseTarballLock{
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
		r := BOSHReleaseTarballSpecification{
			Name:    "capi",
			Version: "~2",
		}
		l := BOSHReleaseTarballLock{
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
		r := BOSHReleaseTarballSpecification{
			Name:    "capi",
			Version: "~2",
		}
		l := BOSHReleaseTarballLock{
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

func Test_checkStemcell_valid(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
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
	please.Expect(results).To(HaveLen(0))
}

func Test_checkStemcell_wrong_version(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
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
	please.Expect(results).To(HaveLen(1))
	please.Expect(results[0]).To(MatchError(ContainSubstring("has stemcell version that does not match the stemcell lock")))
}

func Test_checkStemcell_wrong_os_name(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
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
	please.Expect(results).To(HaveLen(1))
	please.Expect(results[0]).To(MatchError(ContainSubstring("stemcell os that does not match the stemcell lock os")))
}

func Test_checkStemcell_invalid_version_lock(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
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
	please.Expect(results).To(HaveLen(1))
	please.Expect(results[0]).To(MatchError(ContainSubstring("invalid lock version")))
}

func Test_checkStemcell_invalid_version_constraint(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
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
	please.Expect(results).To(HaveLen(1))
	please.Expect(results[0]).To(MatchError(ContainSubstring("invalid version constraint")))
}

func Test_checkStemcell_lock_version_does_not_match_constraint(t *testing.T) {
	t.Parallel()
	please := NewWithT(t)
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
	please.Expect(results).To(HaveLen(1))
	please.Expect(results[0]).To(MatchError(ContainSubstring("does not match constraint")))
}
