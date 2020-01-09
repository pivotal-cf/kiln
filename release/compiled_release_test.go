package release_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/release"
)

var _ = Describe("compiledRelease", func() {
	const (
		expectedName            = "my-awesome-release"
		expectedVersion         = "42.0.0"
		expectedStemcellOS      = "plan9"
		expectedStemcellVersion = "9.9.9"
	)

	DescribeTable("Satisfies", func(name, version, stemcellOS, stemcellVersion string, expectedResult bool) {
		release := NewCompiledRelease(ReleaseID{Name: name, Version: version}, stemcellOS, stemcellVersion)
		requirement := ReleaseRequirement{Name: expectedName, Version: expectedVersion, StemcellOS: expectedStemcellOS, StemcellVersion: expectedStemcellVersion}
		Expect(release.Satisfies(requirement)).To(Equal(expectedResult))
	},
		Entry("when the all attributes match", expectedName, expectedVersion, expectedStemcellOS, expectedStemcellVersion, true),
		Entry("when the release name is different", "wrong-name", expectedVersion, expectedStemcellOS, expectedStemcellVersion, false),
		Entry("when the release version is different", expectedName, "0.0.0", expectedStemcellOS, expectedStemcellVersion, false),
		Entry("when the stemcell os is different", expectedName, expectedVersion, "wrong-os", expectedStemcellVersion, false),
		Entry("when the stemcell version is different", expectedName, expectedVersion, expectedStemcellOS, "0.0.0", false),
	)

	Describe("StandardizedFilename", func() {
		var release DeprecatedRemoteRelease

		BeforeEach(func() {
			release = NewCompiledRelease(
				ReleaseID{Name: expectedName, Version: expectedVersion},
				expectedStemcellOS,
				expectedStemcellVersion,
			)
		})

		It("returns the standardized filename for the release", func() {
			Expect(release.StandardizedFilename()).To(Equal("my-awesome-release-42.0.0-plan9-9.9.9.tgz"))
		})
	})
})
