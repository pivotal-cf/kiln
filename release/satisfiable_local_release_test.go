package release_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/release"
)

var _ = Describe("NewBuiltRelease()", func() {
	const (
		expectedName    = "my-awesome-release"
		expectedVersion = "42.0.0"
	)

	DescribeTable("Satisfies", func(name, version string, expectedResult bool) {
		release := NewBuiltRelease(ReleaseID{Name: name, Version: version}, "not-used")
		requirement := ReleaseRequirement{Name: expectedName, Version: expectedVersion, StemcellOS: "not-used", StemcellVersion: "404"}
		Expect(release.Satisfies(requirement)).To(Equal(expectedResult))
	},
		Entry("when the release name and version match", expectedName, expectedVersion, true),
		Entry("when the release name is different", "something-else", expectedVersion, false),
		Entry("when the release version is different", expectedName, "999.999.999", false),
	)
})

var _ = Describe("NewCompiledRelease()", func() {
	const (
		expectedName            = "my-awesome-release"
		expectedVersion         = "42.0.0"
		expectedStemcellOS      = "plan9"
		expectedStemcellVersion = "9.9.9"
	)

	DescribeTable("Satisfies", func(name, version, stemcellOS, stemcellVersion string, expectedResult bool) {
		release := NewCompiledRelease(ReleaseID{Name: name, Version: version}, stemcellOS, stemcellVersion, "not-used")
		requirement := ReleaseRequirement{Name: expectedName, Version: expectedVersion, StemcellOS: expectedStemcellOS, StemcellVersion: expectedStemcellVersion}
		Expect(release.Satisfies(requirement)).To(Equal(expectedResult))
	},
		Entry("when the all attributes match", expectedName, expectedVersion, expectedStemcellOS, expectedStemcellVersion, true),
		Entry("when the release name is different", "wrong-name", expectedVersion, expectedStemcellOS, expectedStemcellVersion, false),
		Entry("when the release version is different", expectedName, "0.0.0", expectedStemcellOS, expectedStemcellVersion, false),
		Entry("when the stemcell os is different", expectedName, expectedVersion, "wrong-os", expectedStemcellVersion, false),
		Entry("when the stemcell version is different", expectedName, expectedVersion, expectedStemcellOS, "0.0.0", false),
	)
})
