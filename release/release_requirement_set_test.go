package release_test

import (
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/cargo"
	. "github.com/pivotal-cf/kiln/release"
	"github.com/pivotal-cf/kiln/release/fakes"
	"github.com/sclevine/spec"
	"testing"
)

func testReleaseRequirementSet(t *testing.T, context spec.G, it spec.S) {
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

	it.Before(func() {
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

	context("NewReleaseRequirementSet", func() {
		it("constructs a requirement set based on the Kilnfile.lock", func() {
			Expect(rrs).To(HaveLen(2))
			Expect(rrs).To(HaveKeyWithValue(release1ID,
				ReleaseRequirement{Name: release1Name, Version: release1Version, StemcellOS: stemcellName, StemcellVersion: stemcellVersion},
			))
			Expect(rrs).To(HaveKeyWithValue(release2ID,
				ReleaseRequirement{Name: release2Name, Version: release2Version, StemcellOS: stemcellName, StemcellVersion: stemcellVersion},
			))
		})
	})

	context("Partition", func() {
		var (
			releaseSet                             ReleaseWithLocationSet
			extraReleaseID                         ReleaseID
			satisfyingRelease, unsatisfyingRelease *fakes.ReleaseWithLocation
		)

		it.Before(func() {
			satisfyingRelease = new(fakes.ReleaseWithLocation)
			satisfyingRelease.SatisfiesReturns(true)

			unsatisfyingRelease = new(fakes.ReleaseWithLocation)
			unsatisfyingRelease.SatisfiesReturns(false)

			extraReleaseID = ReleaseID{Name: "extra", Version: "2.3.5"}

			releaseSet = ReleaseWithLocationSet{
				release1ID:     satisfyingRelease,
				release2ID:     unsatisfyingRelease,
				extraReleaseID: unsatisfyingRelease,
			}
		})

		it("returns the intersecting, missing, and extra releases", func() {
			intersection, missing, extra := rrs.Partition(releaseSet)

			Expect(intersection).To(HaveLen(1))
			Expect(intersection).To(HaveKeyWithValue(release1ID, satisfyingRelease))

			Expect(missing).To(HaveLen(1))
			Expect(missing).To(HaveKeyWithValue(release2ID, rrs[release2ID]))

			Expect(extra).To(HaveLen(2))
			Expect(extra).To(HaveKeyWithValue(release2ID, unsatisfyingRelease))
			Expect(extra).To(HaveKeyWithValue(extraReleaseID, unsatisfyingRelease))
		})

		it("does not modify itself", func() {
			rrs.Partition(releaseSet)
			Expect(rrs).To(HaveLen(2))
			Expect(rrs).To(HaveKey(release1ID))
			Expect(rrs).To(HaveKey(release2ID))
		})

		it("does not modify the given release set", func() {
			rrs.Partition(releaseSet)
			Expect(releaseSet).To(HaveLen(3))
			Expect(releaseSet).To(HaveKey(release1ID))
			Expect(releaseSet).To(HaveKey(release2ID))
			Expect(releaseSet).To(HaveKey(extraReleaseID))
		})
	})

	context("WithoutReleases", func() {
		it("returns a set without those releases", func() {
			release2Requirement := rrs[release2ID]
			result := rrs.WithoutReleases([]ReleaseID{release1ID})

			Expect(result).To(HaveLen(1))
			Expect(result).NotTo(HaveKey(release1ID))
			Expect(result).To(HaveKeyWithValue(release2ID, release2Requirement))
		})

		it("does not modify the original", func() {
			_ = rrs.WithoutReleases([]ReleaseID{release1ID})
			Expect(rrs).To(HaveLen(2))
			Expect(rrs).To(HaveKey(release1ID))
		})
	})
}
