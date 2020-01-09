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
