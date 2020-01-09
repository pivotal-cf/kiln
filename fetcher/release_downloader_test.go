package fetcher_test

import (
	"errors"
	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/commands"
	. "github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/fetcher/fakes"
	"github.com/pivotal-cf/kiln/release"
)

var _ = Describe("DownloadRelease", func() {
	const (
		releaseName    = "evangelion"
		releaseVersion = "3.33"
		remotePath     = "something-remote.tgz"
	)
	var (
		releaseDownloader                            commands.ReleaseDownloader
		primaryReleaseSource, secondaryReleaseSource *fakes.ReleaseSource
		downloadDir                                  string
		requirement                                  release.ReleaseRequirement
		releaseID                                    release.ReleaseID
		expectedRemoteRelease                        release.RemoteRelease
		expectedLocalRelease                         release.LocalRelease
	)

	BeforeEach(func() {
		primaryReleaseSource = new(fakes.ReleaseSource)
		primaryReleaseSource.IDReturns("primary")
		secondaryReleaseSource = new(fakes.ReleaseSource)
		secondaryReleaseSource.IDReturns("secondary")

		releaseDownloader = NewReleaseDownloader([]ReleaseSource{primaryReleaseSource, secondaryReleaseSource})

		var err error
		downloadDir, err = ioutil.TempDir("/tmp", "download-release-spec")
		Expect(err).NotTo(HaveOccurred())

		requirement = release.ReleaseRequirement{
			Name:            releaseName,
			Version:         releaseVersion,
			StemcellOS:      "magi",
			StemcellVersion: "3",
		}

		releaseID = release.ReleaseID{Name: releaseName, Version: releaseVersion}
		expectedRemoteRelease = release.RemoteRelease{ReleaseID: releaseID, RemotePath: remotePath}
		expectedLocalRelease = release.LocalRelease{ReleaseID: releaseID, LocalPath: filepath.Join(downloadDir, "evangelion-3.33.tgz")}
	})

	When("the release is available from the primary release source", func() {
		BeforeEach(func() {
			primaryReleaseSource.GetMatchedReleasesReturns([]release.RemoteRelease{expectedRemoteRelease}, nil)
			primaryReleaseSource.DownloadReleasesReturns([]release.LocalRelease{expectedLocalRelease}, nil)
		})

		It("downloads the release from that source", func() {
			localRelease, remoteSource, remotePath, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).NotTo(HaveOccurred())
			Expect(localRelease).To(Equal(expectedLocalRelease))
			Expect(remoteSource).To(Equal(primaryReleaseSource.ID()))
			Expect(remotePath).To(Equal(remotePath))

			Expect(primaryReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
			Expect(secondaryReleaseSource.DownloadReleasesCallCount()).To(Equal(0))

			actualDir, actualRemoteReleases, _ := primaryReleaseSource.DownloadReleasesArgsForCall(0)
			Expect(actualDir).To(Equal(downloadDir))
			Expect(actualRemoteReleases).To(ConsistOf(expectedRemoteRelease))
		})
	})

	When("the release is available from the secondary release source", func() {
		BeforeEach(func() {
			primaryReleaseSource.GetMatchedReleasesReturns([]release.RemoteRelease{}, nil)
			secondaryReleaseSource.GetMatchedReleasesReturns([]release.RemoteRelease{expectedRemoteRelease}, nil)
			secondaryReleaseSource.DownloadReleasesReturns([]release.LocalRelease{expectedLocalRelease}, nil)
		})

		It("downloads the release from that source", func() {
			localRelease, remoteSource, remotePath, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).NotTo(HaveOccurred())
			Expect(localRelease).To(Equal(expectedLocalRelease))
			Expect(remoteSource).To(Equal(secondaryReleaseSource.ID()))
			Expect(remotePath).To(Equal(remotePath))

			Expect(primaryReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
			Expect(secondaryReleaseSource.DownloadReleasesCallCount()).To(Equal(1))

			actualDir, actualRemoteReleases, _ := secondaryReleaseSource.DownloadReleasesArgsForCall(0)
			Expect(actualDir).To(Equal(downloadDir))
			Expect(actualRemoteReleases).To(ConsistOf(expectedRemoteRelease))
		})
	})

	When("the release isn't available from any release source", func() {
		BeforeEach(func() {
			primaryReleaseSource.GetMatchedReleasesReturns([]release.RemoteRelease{}, nil)
			secondaryReleaseSource.GetMatchedReleasesReturns([]release.RemoteRelease{}, nil)
		})

		It("errors and doesn't download", func() {
			_, _, _, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).To(MatchError("couldn't find \"evangelion\" 3.33 in any release source"))
		})

		It("doesn't download", func() {
			releaseDownloader.DownloadRelease(downloadDir, requirement)

			Expect(primaryReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
			Expect(secondaryReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
		})
	})

	When("there's an error finding a matching release", func() {
		var expectedError error

		BeforeEach(func() {
			expectedError = errors.New("boom")
			primaryReleaseSource.GetMatchedReleasesReturns(nil, expectedError)
		})

		It("returns that error", func() {
			_, _, _, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).To(MatchError(expectedError))
		})

		It("doesn't download anything", func() {
			releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(primaryReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
			Expect(secondaryReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
		})
	})

	When("there's an error downloading the release", func() {
		var expectedError error

		BeforeEach(func() {
			expectedError = errors.New("boom")
			primaryReleaseSource.GetMatchedReleasesReturns([]release.RemoteRelease{expectedRemoteRelease}, nil)
			primaryReleaseSource.DownloadReleasesReturns(nil, expectedError)
		})

		It("returns that error", func() {
			_, _, _, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).To(MatchError(expectedError))
		})
	})
})
