package release_test

import (
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/release"
	"github.com/sclevine/spec"
	"strconv"
	"testing"
)

func testCompiledRelease(t *testing.T, context spec.G, it spec.S) {
	const (
		expectedName            = "my-awesome-release"
		expectedVersion         = "42.0.0"
		expectedStemcellOS      = "plan9"
		expectedStemcellVersion = "9.9.9"
	)

	context("Satisfies", func() {
		scenario := func(description, name, version, stemcellOS, stemcellVersion string, expectedResult bool) {
			context(description, func() {
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

	context("StandardizedFilename", func() {
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
}
