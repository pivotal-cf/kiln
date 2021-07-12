package fetcher_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/fetcher"
	fetcherFakes "github.com/pivotal-cf/kiln/internal/fetcher/fakes"
	"github.com/pivotal-cf/kiln/pkg/release"
)

var _ = Describe("multiReleaseSource", func() {
	var (
		multiSrc         fetcher.MultiReleaseSource
		src1, src2, src3 *fetcherFakes.ReleaseSource
		requirement      release.Requirement
	)

	const (
		releaseName         = "stuff-and-things"
		releaseVersion      = "42.42"
		releaseVersionNewer = "43.43"
	)

	BeforeEach(func() {
		src1 = new(fetcherFakes.ReleaseSource)
		src1.IDReturns("src-1")
		src2 = new(fetcherFakes.ReleaseSource)
		src2.IDReturns("src-2")
		src3 = new(fetcherFakes.ReleaseSource)
		src3.IDReturns("src-3")
		multiSrc = fetcher.NewMultiReleaseSource(src1, src2, src3)

		requirement = release.Requirement{
			Name:            releaseName,
			Version:         releaseVersion,
			StemcellOS:      "not-used",
			StemcellVersion: "not-used",
		}
	})

	Describe("GetMatchedRelease", func() {
		When("one of the release sources has a match", func() {
			var (
				matchedRelease release.Remote
			)

			BeforeEach(func() {
				matchedRelease = release.Remote{
					ID:         release.ID{Name: releaseName, Version: releaseVersion},
					RemotePath: "/some/path",
					SourceID:   src2.ID(),
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
				src2.GetMatchedReleaseReturns(release.Remote{}, false, expectedErr)
			})

			It("returns that error", func() {
				_, found, err := multiSrc.GetMatchedRelease(requirement)
				Expect(err).To(MatchError(ContainSubstring(src2.ID())))
				Expect(err).To(MatchError(ContainSubstring(expectedErr.Error())))
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("DownloadRelease", func() {
		var (
			releaseID release.ID
			remote    *release.Remote
		)

		BeforeEach(func() {
			releaseID = release.ID{Name: releaseName, Version: releaseVersion}
			remote = &release.Remote{ID: releaseID, RemotePath: "/some/remote/path", SourceID: src2.ID()}
		})

		When("the source exists and downloads without error", func() {
			var local release.Local

			BeforeEach(func() {
				local = release.Local{ID: releaseID, LocalPath: "somewhere/on/disk", SHA1: "a-sha1"}
				src2.DownloadReleaseReturns(local, nil)
			})

			It("returns the local release", func() {
				l, err := multiSrc.DownloadRelease("somewhere", *remote, 42)
				Expect(err).NotTo(HaveOccurred())
				Expect(l).To(Equal(local))

				Expect(src2.DownloadReleaseCallCount()).To(Equal(1))
				dir, r, threads := src2.DownloadReleaseArgsForCall(0)
				Expect(dir).To(Equal("somewhere"))
				Expect(r).To(Equal(*remote))
				Expect(threads).To(Equal(42))
			})
		})

		When("the source exists and the download errors", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("big badda boom")
				src2.DownloadReleaseReturns(release.Local{}, expectedErr)
			})

			It("returns the error", func() {
				_, err := multiSrc.DownloadRelease("somewhere", *remote, 42)
				Expect(err).To(MatchError(ContainSubstring(src2.ID())))
				Expect(err).To(MatchError(ContainSubstring(expectedErr.Error())))
			})
		})

		When("the source doesn't exist", func() {
			BeforeEach(func() {
				remote.SourceID = "no-such-source"
			})

			It("errors", func() {
				_, err := multiSrc.DownloadRelease("somewhere", *remote, 42)
				Expect(err).To(MatchError(ContainSubstring("couldn't find a release source")))
				Expect(err).To(MatchError(ContainSubstring("no-such-source")))
				Expect(err).To(MatchError(ContainSubstring(src1.ID())))
				Expect(err).To(MatchError(ContainSubstring(src2.ID())))
				Expect(err).To(MatchError(ContainSubstring(src3.ID())))
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
				matchedRelease release.Remote
			)

			BeforeEach(func() {
				matchedRelease = release.Remote{
					ID:         release.ID{Name: releaseName, Version: releaseVersion},
					RemotePath: "/some/path",
					SourceID:   src2.ID(),
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
				matchedRelease release.Remote
			)

			BeforeEach(func() {
				unmatchedRelease := release.Remote{
					ID:         release.ID{Name: releaseName, Version: releaseVersion},
					RemotePath: "/some/path",
					SourceID:   src1.ID(),
				}
				matchedRelease = release.Remote{
					ID:         release.ID{Name: releaseName, Version: releaseVersionNewer},
					RemotePath: "/some/path",
					SourceID:   src2.ID(),
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
				matchedRelease release.Remote
			)

			BeforeEach(func() {
				matchedRelease = release.Remote{
					ID:         release.ID{Name: releaseName, Version: releaseVersion},
					RemotePath: "/some/path",
					SourceID:   src1.ID(),
				}
				unmatchedRelease := release.Remote{
					ID:         release.ID{Name: releaseName, Version: releaseVersion},
					RemotePath: "/some/path",
					SourceID:   src2.ID(),
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
