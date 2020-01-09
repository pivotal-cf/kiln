package release_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/release"
)

var _ = Describe("builtRelease", func() {
	const (
		expectedName    = "my-awesome-release"
		expectedVersion = "42.0.0"
	)

	DescribeTable("Satisfies", func(name, version string, expectedResult bool) {
		release := NewBuiltRelease(ReleaseID{Name: name, Version: version})
		requirement := ReleaseRequirement{Name: expectedName, Version: expectedVersion, StemcellOS: "not-used", StemcellVersion: "404"}
		Expect(release.Satisfies(requirement)).To(Equal(expectedResult))
	},
		Entry("when the release name and version match", expectedName, expectedVersion, true),
		Entry("when the release name is different", "something-else", expectedVersion, false),
		Entry("when the release version is different", expectedName, "999.999.999", false),
	)

	Describe("StandardizedFilename", func() {
		var release DeprecatedRemoteRelease

		BeforeEach(func() {
			release = NewBuiltRelease(ReleaseID{Name: expectedName, Version: expectedVersion})
		})

		It("returns the standardized filename for the release", func() {
			Expect(release.StandardizedFilename()).To(Equal("my-awesome-release-42.0.0.tgz"))
		})
	})
})
