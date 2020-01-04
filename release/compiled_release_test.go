package release_test

import (
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/release"
	"github.com/sclevine/spec"
	"strconv"
	"testing"
)

func TestCompiledRelease(t *testing.T) {
	spec.Run(t, "compiledRelease", func(t *testing.T, when spec.G, it spec.S) {
		const (
			expectedName            = "my-awesome-release"
			expectedVersion         = "42.0.0"
			expectedStemcellOS      = "plan9"
			expectedStemcellVersion = "9.9.9"
		)

		it.Before(func() {
			RegisterTestingT(t)
		})

		when("StandardizedFilename", func() {
			var release RemoteRelease

			it.Before(func() {
				release = NewCompiledRelease(
					ReleaseID{Name: expectedName, Version: expectedVersion},
					expectedStemcellOS,
					expectedStemcellVersion,
				)
			})

			it("returns the standardized filename for the release", func() {
				Expect(release.StandardizedFilename()).To(Equal("my-awesome-release-42.0.0-plan9-9.9.9.tgz"))
			})
		})

		when("Satisfies", func() {
			scenario := func(description, name, version, stemcellOS, stemcellVersion string, expectedResult bool) {
				when(description, func() {
					it("is "+strconv.FormatBool(expectedResult), func() {
						release := NewCompiledRelease(ReleaseID{Name: name, Version: version}, stemcellOS, stemcellVersion)
						requirement := ReleaseRequirement{Name: expectedName, Version: expectedVersion, StemcellOS: expectedStemcellOS, StemcellVersion: expectedStemcellVersion}
						Expect(release.Satisfies(requirement)).To(Equal(expectedResult))
					})
				})
			}
			scenario("the all attributes match", expectedName, expectedVersion, expectedStemcellOS, expectedStemcellVersion, true)
			scenario("the release name is different", "wrong-name", expectedVersion, expectedStemcellOS, expectedStemcellVersion, false)
			scenario("the release version is different", expectedName, "0.0.0", expectedStemcellOS, expectedStemcellVersion, false)
			scenario("the stemcell os is different", expectedName, expectedVersion, "wrong-os", expectedStemcellVersion, false)
			scenario("the stemcell version is different", expectedName, expectedVersion, expectedStemcellOS, "0.0.0", false)
		})
	})
}
