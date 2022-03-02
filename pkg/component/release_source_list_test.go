package component_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/component"
	"github.com/pivotal-cf/kiln/pkg/component/fakes"
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
				src1.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
				src3.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
				src2.GetMatchedReleaseReturns(matchedRelease, nil)
			})

			It("returns that match", func() {
				rel, err := multiSrc.GetMatchedRelease(requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(rel).To(Equal(matchedRelease))
			})
		})

		When("none of the release sources has a match", func() {
			BeforeEach(func() {
				src1.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
				src3.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
				src2.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
			})
			It("returns no match", func() {
				_, err := multiSrc.GetMatchedRelease(requirement)
				Expect(err).To(HaveOccurred())
				Expect(component.IsErrNotFound(err)).To(BeTrue())
			})
		})

		When("one of the release sources errors", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("bad stuff happened")
				src1.GetMatchedReleaseReturns(component.Lock{}, expectedErr)
			})

			It("returns that error", func() {
				_, err := multiSrc.GetMatchedRelease(requirement)
				Expect(err).To(MatchError(ContainSubstring(src1.Configuration().ID)))
				Expect(err).To(MatchError(ContainSubstring(expectedErr.Error())))
			})
		})
	})

	Describe("DownloadRelease", func() {
		var (
			releaseID component.Spec
			remote    component.Lock
		)

		BeforeEach(func() {
			releaseID = component.Spec{Name: releaseName, Version: releaseVersion}
			remote = releaseID.Lock().WithRemote(src2.Configuration().ID, "/some/remote/path")
		})

		When("the source exists and downloads without error", func() {
			var local component.Local

			BeforeEach(func() {
				l := releaseID.Lock()
				l.SHA1 = "a-sha1"
				local = component.Local{Lock: releaseID.Lock(), LocalPath: "somewhere/on/disk"}
				src2.DownloadReleaseReturns(local, nil)
			})

			It("returns the local release", func() {
				l, err := multiSrc.DownloadRelease("somewhere", remote)
				Expect(err).NotTo(HaveOccurred())
				Expect(l).To(Equal(local))

				Expect(src2.DownloadReleaseCallCount()).To(Equal(1))
				dir, r := src2.DownloadReleaseArgsForCall(0)
				Expect(dir).To(Equal("somewhere"))
				Expect(r).To(Equal(remote))
			})
		})

		When("the source exists and the download errors", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("big badda boom")
				src2.DownloadReleaseReturns(component.Local{}, expectedErr)
			})

			It("returns the error", func() {
				_, err := multiSrc.DownloadRelease("somewhere", remote)
				Expect(err).To(MatchError(ContainSubstring(src2.Configuration().ID)))
				Expect(err).To(MatchError(ContainSubstring(expectedErr.Error())))
			})
		})

		When("the source doesn't exist", func() {
			BeforeEach(func() {
				remote.RemoteSource = "no-such-source"
			})

			It("errors", func() {
				_, err := multiSrc.DownloadRelease("somewhere", remote)
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
				src1.FindReleaseVersionReturns(component.Lock{}, component.ErrNotFound)
				src2.FindReleaseVersionReturns(matchedRelease, nil)
				src3.FindReleaseVersionReturns(component.Lock{}, component.ErrNotFound)
			})

			It("returns that match", func() {
				rel, err := multiSrc.FindReleaseVersion(requirement)
				Expect(err).NotTo(HaveOccurred())
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
				src1.FindReleaseVersionReturns(unmatchedRelease, nil)
				src2.FindReleaseVersionReturns(matchedRelease, nil)
				src3.FindReleaseVersionReturns(component.Lock{}, component.ErrNotFound)
			})

			It("returns that match", func() {
				rel, err := multiSrc.FindReleaseVersion(requirement)
				Expect(err).NotTo(HaveOccurred())
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
				src1.FindReleaseVersionReturns(matchedRelease, nil)
				src2.FindReleaseVersionReturns(unmatchedRelease, nil)
				src3.FindReleaseVersionReturns(component.Lock{}, component.ErrNotFound)
			})

			It("returns the match from the first source", func() {
				rel, err := multiSrc.FindReleaseVersion(requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(rel).To(Equal(matchedRelease))
			})
		})
	})
})
