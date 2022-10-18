package component_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("multiReleaseSource", func() {
	var (
		multiSrc          component.MultiReleaseSource
		src1, src2, src3  *fakes.ReleaseSource
		src1IsPublishable = true
	)

	const (
		releaseName         = "stuff-and-things"
		releaseVersion      = "42.42"
		releaseVersionNewer = "43.43"
	)

	JustBeforeEach(func() {
		src1 = new(fakes.ReleaseSource)
		src1.ConfigurationReturns(cargo.ReleaseSourceConfig{ID: "src-1", Publishable: src1IsPublishable})
		src2 = new(fakes.ReleaseSource)
		src2.ConfigurationReturns(cargo.ReleaseSourceConfig{ID: "src-2"})
		src3 = new(fakes.ReleaseSource)
		src3.ConfigurationReturns(cargo.ReleaseSourceConfig{ID: "src-3"})
		multiSrc = component.NewMultiReleaseSource(src1, src2, src3)
	})

	Describe("FindByID", func() {
		When("the source exists", func() {
			It("returns it", func() {
				match, err := multiSrc.FindByID("src-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(Equal(src1))

				match, err = multiSrc.FindByID("src-2")
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(Equal(src2))

				match, err = multiSrc.FindByID("src-3")
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(Equal(src3))
			})
		})

		When("the source doesn't exist", func() {
			It("errors", func() {
				_, err := multiSrc.FindByID("no-such-source")
				Expect(err).To(MatchError(ContainSubstring("not found")))
				Expect(err).To(MatchError(ContainSubstring("no-such-source")))

				Expect(err).To(MatchError(ContainSubstring("src-1")))
				Expect(err).To(MatchError(ContainSubstring("src-2")))
				Expect(err).To(MatchError(ContainSubstring("src-3")))
			})
		})
	})

	Describe("GetReleaseCache", func() {
		When("a source is publishable", func() {
			It("returns it", func() {
				match, err := multiSrc.GetReleaseCache()
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(Equal(src1))
			})
		})

		When("no source is publishable", func() {
			BeforeEach(func() {
				src1IsPublishable = false
			})
			It("errors", func() {
				match, err := multiSrc.GetReleaseCache()
				Expect(err).To(HaveOccurred())
				Expect(match).To(BeNil())
			})
		})
	})
})
