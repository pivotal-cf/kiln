package component_test

import (
	"context"
	"errors"
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("multiReleaseSource", func() {
	var (
		multiSrc         component.MultiReleaseSource
		src1, src2, src3 *fakes.ReleaseSource
		requirement      component.Spec
	)

	const (
		releaseName         = "stuff-and-things"
		releaseVersion      = "42.42"
		releaseVersionNewer = "43.43"
	)

	BeforeEach(func() {
		src1 = new(fakes.ReleaseSource)
		src1.ConfigurationReturns(cargo.ReleaseSourceConfig{ID: "src-1"})
		src2 = new(fakes.ReleaseSource)
		src2.ConfigurationReturns(cargo.ReleaseSourceConfig{ID: "src-2"})
		src3 = new(fakes.ReleaseSource)
		src3.ConfigurationReturns(cargo.ReleaseSourceConfig{ID: "src-3"})
		multiSrc = component.NewMultiReleaseSource(src1, src2, src3)

		requirement = component.Spec{
			Name:            releaseName,
			Version:         releaseVersion,
			StemcellOS:      "not-used",
			StemcellVersion: "not-used",
		}
	})

	Describe("GetMatchedRelease", func() {
		When("one of the release sources has a match", func() {
			var (
				matchedRelease component.Lock
			)

			BeforeEach(func() {
				matchedRelease = component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src2.Configuration().ID,
				}
				src2.GetMatchedReleaseReturns(matchedRelease, true, nil)
			})

			It("returns that match", func() {
				rel, found, err := multiSrc.GetMatchedRelease(requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(rel).To(Equal(matchedRelease))
			})
		})

		When("none of the release sources has a match", func() {
			It("returns no match", func() {
				_, found, err := multiSrc.GetMatchedRelease(requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		When("one of the release sources errors", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("bad stuff happened")
				src2.GetMatchedReleaseReturns(component.Lock{}, false, expectedErr)
			})

			It("returns that error", func() {
				_, found, err := multiSrc.GetMatchedRelease(requirement)
				Expect(err).To(MatchError(ContainSubstring(src2.Configuration().ID)))
				Expect(err).To(MatchError(ContainSubstring(expectedErr.Error())))
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("DownloadRelease", func() {
		var remote component.Lock

		BeforeEach(func() {
			remote = component.Lock{Name: releaseName, Version: releaseVersion}.
				WithRemote(src2.Configuration().ID, "/some/remote/path")
		})

		When("the source exists and downloads without error", func() {
			BeforeEach(func() {
				src2.DownloadComponentReturns(nil)
			})

			It("returns without an error", func() {
				err := multiSrc.DownloadComponent(context.Background(), io.Discard, remote)
				Expect(err).NotTo(HaveOccurred())

				Expect(src2.DownloadComponentCallCount()).To(Equal(1))
				ctx, w, r := src2.DownloadComponentArgsForCall(0)
				Expect(ctx).NotTo(BeNil())
				Expect(w).NotTo(BeNil())
				Expect(r).To(Equal(remote))
			})
		})

		When("the source exists and the download errors", func() {
			const expectedErrMessage = "big badda boom"
			BeforeEach(func() {
				src2.DownloadComponentReturns(errors.New(expectedErrMessage))
			})

			It("returns the error", func() {
				err := multiSrc.DownloadComponent(context.Background(), io.Discard, remote)
				Expect(err).To(MatchError(ContainSubstring(src2.Configuration().ID)))
				Expect(err).To(MatchError(ContainSubstring(expectedErrMessage)))
			})
		})

		When("the source doesn't exist", func() {
			BeforeEach(func() {
				remote.RemoteSource = "no-such-source"
			})

			It("errors", func() {
				err := multiSrc.DownloadComponent(context.Background(), io.Discard, remote)
				Expect(err).To(MatchError(ContainSubstring("couldn't find a release source")))
				Expect(err).To(MatchError(ContainSubstring("no-such-source")))
				Expect(err).To(MatchError(ContainSubstring(src1.Configuration().ID)))
				Expect(err).To(MatchError(ContainSubstring(src2.Configuration().ID)))
				Expect(err).To(MatchError(ContainSubstring(src3.Configuration().ID)))
			})
		})
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
				Expect(err).To(MatchError(ContainSubstring("couldn't find")))
				Expect(err).To(MatchError(ContainSubstring("no-such-source")))

				Expect(err).To(MatchError(ContainSubstring("src-1")))
				Expect(err).To(MatchError(ContainSubstring("src-2")))
				Expect(err).To(MatchError(ContainSubstring("src-3")))
			})
		})
	})

	Describe("FindReleaseVersion", func() {
		When("one of the release sources has a match", func() {
			var (
				matchedRelease component.Lock
			)

			BeforeEach(func() {
				matchedRelease = component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src2.Configuration().ID,
				}
				src2.FindReleaseVersionReturns(matchedRelease, true, nil)
			})

			It("returns that match", func() {
				rel, found, err := multiSrc.FindReleaseVersion(requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(rel).To(Equal(matchedRelease))
			})
		})
		When("two of the release sources have a match", func() {
			var (
				matchedRelease component.Lock
			)

			BeforeEach(func() {
				unmatchedRelease := component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src1.Configuration().ID,
				}
				matchedRelease = component.Lock{
					Name:         releaseName,
					Version:      releaseVersionNewer,
					RemotePath:   "/some/path",
					RemoteSource: src2.Configuration().ID,
				}
				src1.FindReleaseVersionReturns(unmatchedRelease, true, nil)
				src2.FindReleaseVersionReturns(matchedRelease, true, nil)
			})

			It("returns that match", func() {
				rel, found, err := multiSrc.FindReleaseVersion(requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(rel).To(Equal(matchedRelease))
			})
		})
		When("two of the release sources match the same version", func() {
			var (
				matchedRelease component.Lock
			)

			BeforeEach(func() {
				matchedRelease = component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src1.Configuration().ID,
				}
				unmatchedRelease := component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src2.Configuration().ID,
				}
				src1.FindReleaseVersionReturns(matchedRelease, true, nil)
				src2.FindReleaseVersionReturns(unmatchedRelease, true, nil)
			})

			It("returns the match from the first source", func() {
				rel, found, err := multiSrc.FindReleaseVersion(requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(rel).To(Equal(matchedRelease))
			})
		})
	})
})
