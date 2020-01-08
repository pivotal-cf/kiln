package release_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/cargo"
	. "github.com/pivotal-cf/kiln/release"
)

var _ = Describe("ReleaseRequirementSet", func() {
	const (
		release1Name    = "release-1"
		release1Version = "1.2.3"
		release2Name    = "release-2"
		release2Version = "2.3.4"
		stemcellName    = "some-os"
		stemcellVersion = "9.8.7"
	)

	var (
		rrs                    ReleaseRequirementSet
		release1ID, release2ID ReleaseID
	)

	BeforeEach(func() {
		kilnfileLock := cargo.KilnfileLock{
			Releases: []cargo.ReleaseLock{
				{Name: release1Name, Version: release1Version},
				{Name: release2Name, Version: release2Version},
			},
			Stemcell: cargo.Stemcell{OS: stemcellName, Version: stemcellVersion},
		}
		rrs = NewReleaseRequirementSet(kilnfileLock)
		release1ID = ReleaseID{Name: release1Name, Version: release1Version}
		release2ID = ReleaseID{Name: release2Name, Version: release2Version}
	})

	Describe("NewReleaseRequirementSet", func() {
		It("constructs a requirement set based on the Kilnfile.lock", func() {
			Expect(rrs).To(HaveLen(2))
			Expect(rrs).To(HaveKeyWithValue(release1ID,
				ReleaseRequirement{Name: release1Name, Version: release1Version, StemcellOS: stemcellName, StemcellVersion: stemcellVersion},
			))
			Expect(rrs).To(HaveKeyWithValue(release2ID,
				ReleaseRequirement{Name: release2Name, Version: release2Version, StemcellOS: stemcellName, StemcellVersion: stemcellVersion},
			))
		})
	})

	Describe("Partition", func() {
		var (
			releaseSet                             ReleaseWithLocationSet
			extraReleaseID                         ReleaseID
			satisfyingRelease, unsatisfyingRelease, extraRelease ReleaseWithLocation
		)

		BeforeEach(func() {
			satisfyingRelease = NewBuiltRelease(release1ID).WithLocalPath("satisfying-path")
			unsatisfyingRelease = NewBuiltRelease(ReleaseID{Name: release2Name, Version: "4.0.4"}).WithLocalPath("unsatisfying-path")

			extraReleaseID = ReleaseID{Name: "extra", Version: "2.3.5"}
			extraRelease = NewBuiltRelease(extraReleaseID).WithLocalPath("so-extra")

			releaseSet = ReleaseWithLocationSet{
				release1ID:     satisfyingRelease,
				release2ID:     unsatisfyingRelease,
				extraReleaseID: extraRelease,
			}
		})

		It("returns the intersecting, missing, and extra releases", func() {
			intersection, missing, extra := rrs.Partition(releaseSet)

			Expect(intersection).To(HaveLen(1))
			Expect(intersection).To(HaveKeyWithValue(
				release1ID,
				LocalRelease{ReleaseID: release1ID, LocalPath: satisfyingRelease.LocalPath()},
			))

			Expect(missing).To(HaveLen(1))
			Expect(missing).To(HaveKeyWithValue(release2ID, rrs[release2ID]))

			Expect(extra).To(HaveLen(2))
			Expect(extra).To(HaveKeyWithValue(
				release2ID,
				LocalRelease{
					ReleaseID: ReleaseID{Name: release2Name, Version: "4.0.4"},
					LocalPath: unsatisfyingRelease.LocalPath(),
				},
			))
			Expect(extra).To(HaveKeyWithValue(
				extraReleaseID,
				LocalRelease{ReleaseID: extraReleaseID, LocalPath: extraRelease.LocalPath()},
			))
		})

		It("does not modify itself", func() {
			rrs.Partition(releaseSet)
			Expect(rrs).To(HaveLen(2))
			Expect(rrs).To(HaveKey(release1ID))
			Expect(rrs).To(HaveKey(release2ID))
		})

		It("does not modify the given release set", func() {
			rrs.Partition(releaseSet)
			Expect(releaseSet).To(HaveLen(3))
			Expect(releaseSet).To(HaveKey(release1ID))
			Expect(releaseSet).To(HaveKey(release2ID))
			Expect(releaseSet).To(HaveKey(extraReleaseID))
		})
	})

	Describe("WithoutReleases", func() {
		It("returns a set without those releases", func() {
			release2Requirement := rrs[release2ID]
			result := rrs.WithoutReleases([]ReleaseID{release1ID})

			Expect(result).To(HaveLen(1))
			Expect(result).NotTo(HaveKey(release1ID))
			Expect(result).To(HaveKeyWithValue(release2ID, release2Requirement))
		})

		It("does not modify the original", func() {
			_ = rrs.WithoutReleases([]ReleaseID{release1ID})
			Expect(rrs).To(HaveLen(2))
			Expect(rrs).To(HaveKey(release1ID))
		})
	})
})
