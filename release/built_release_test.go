package release_test

import (
	"strconv"

	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/release"
	"github.com/sclevine/spec"
	"testing"
)

func TestBuiltRelease(t *testing.T) {
	spec.Run(t, "builtRelease", func(t *testing.T, when spec.G, it spec.S) {
		const (
			expectedName    = "my-awesome-release"
			expectedVersion = "42.0.0"
		)

		it.Before(func() {
			RegisterTestingT(t)
		})

		when("StandardizedFilename", func() {
			var release RemoteRelease

			it.Before(func() {
				release = NewBuiltRelease(ReleaseID{Name: expectedName, Version: expectedVersion})
			})

			it("returns the standardized filename for the release", func() {
				Expect(release.StandardizedFilename()).To(Equal("my-awesome-release-42.0.0.tgz"))
			})
		})

		when("Satisfies", func() {
			scenario := func(description, name, version string, expectedResult bool) {
				when(description, func() {
					it("is " + strconv.FormatBool(expectedResult), func() {
						release := NewBuiltRelease(ReleaseID{Name: name, Version: version})
						requirement := ReleaseRequirement{Name: expectedName, Version: expectedVersion, StemcellOS: "not-used", StemcellVersion: "404"}
						Expect(release.Satisfies(requirement)).To(Equal(expectedResult))
					})
				})
			}
			scenario("the release name and version match", expectedName, expectedVersion, true)
			scenario("the release name is different", "something-else", expectedVersion, false)
			scenario("the release version is different", expectedName, "999.999.999", false)
		})
	})
}
