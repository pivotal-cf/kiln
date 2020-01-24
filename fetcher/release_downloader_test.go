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
		primaryReleaseSource, secondaryReleaseSource *fakes.ReleaseSourceWithID
		downloadDir                                  string
		requirement                                  release.Requirement
		releaseID                                    release.ID
		expectedRemoteRelease                        *release.Remote
		expectedLocalRelease                         release.Local
	)

	BeforeEach(func() {
		primaryReleaseSource = new(fakes.ReleaseSourceWithID)
		primaryReleaseSource.IDReturns("primary")
		secondaryReleaseSource = new(fakes.ReleaseSourceWithID)
		secondaryReleaseSource.IDReturns("secondary")

		releaseDownloader = NewReleaseDownloader(MultiReleaseSource{primaryReleaseSource, secondaryReleaseSource})

		var err error
		downloadDir, err = ioutil.TempDir("/tmp", "download-release-spec")
		Expect(err).NotTo(HaveOccurred())

		requirement = release.Requirement{
			Name:            releaseName,
			Version:         releaseVersion,
			StemcellOS:      "magi",
			StemcellVersion: "3",
		}

		releaseID = release.ID{Name: releaseName, Version: releaseVersion}
		expectedRemoteRelease = &release.Remote{ID: releaseID, RemotePath: remotePath}
		expectedLocalRelease = release.Local{ID: releaseID, LocalPath: filepath.Join(downloadDir, "evangelion-3.33.tgz")}
	})

	When("the release is available from the primary release source", func() {
		BeforeEach(func() {
			expectedRemoteRelease.SourceID = primaryReleaseSource.ID()
			primaryReleaseSource.GetMatchedReleaseReturns(*expectedRemoteRelease, true, nil)
			primaryReleaseSource.DownloadReleaseReturns(expectedLocalRelease, nil)
		})

		It("downloads the release from that source", func() {
			localRelease, remoteRelease, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).NotTo(HaveOccurred())
			Expect(localRelease).To(Equal(expectedLocalRelease))
			Expect(remoteRelease).To(Equal(*expectedRemoteRelease))
			Expect(remotePath).To(Equal(remotePath))

			Expect(primaryReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
			Expect(secondaryReleaseSource.DownloadReleaseCallCount()).To(Equal(0))

			actualDir, actualRemoteRelease, _ := primaryReleaseSource.DownloadReleaseArgsForCall(0)
			Expect(actualDir).To(Equal(downloadDir))
			Expect(actualRemoteRelease).To(Equal(*expectedRemoteRelease))
		})
	})

	When("the release is available from the secondary release source", func() {
		BeforeEach(func() {
			expectedRemoteRelease.SourceID = secondaryReleaseSource.ID()
			primaryReleaseSource.GetMatchedReleaseReturns(release.Remote{}, false, nil)
			secondaryReleaseSource.GetMatchedReleaseReturns(*expectedRemoteRelease, true, nil)
			secondaryReleaseSource.DownloadReleaseReturns(expectedLocalRelease, nil)
		})

		It("downloads the release from that source", func() {
			localRelease, remoteRelease, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).NotTo(HaveOccurred())
			Expect(localRelease).To(Equal(expectedLocalRelease))
			Expect(remoteRelease).To(Equal(*expectedRemoteRelease))

			Expect(primaryReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
			Expect(secondaryReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

			actualDir, actualRemoteRelease, _ := secondaryReleaseSource.DownloadReleaseArgsForCall(0)
			Expect(actualDir).To(Equal(downloadDir))
			Expect(actualRemoteRelease).To(Equal(*expectedRemoteRelease))
		})
	})

	When("the release isn't available from any release source", func() {
		BeforeEach(func() {
			primaryReleaseSource.GetMatchedReleaseReturns(release.Remote{}, false, nil)
			secondaryReleaseSource.GetMatchedReleaseReturns(release.Remote{}, false, nil)
		})

		It("errors and doesn't download", func() {
			_, _, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).To(MatchError("couldn't find \"evangelion\" 3.33 in any release source"))
		})

		It("doesn't download", func() {
			releaseDownloader.DownloadRelease(downloadDir, requirement)

			Expect(primaryReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
			Expect(secondaryReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
		})
	})

	When("there's an error finding a matching release", func() {
		var expectedError error

		BeforeEach(func() {
			expectedError = errors.New("boom")
			primaryReleaseSource.GetMatchedReleaseReturns(release.Remote{}, false, expectedError)
		})

		It("returns that error", func() {
			_, _, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).To(MatchError(expectedError))
		})

		It("doesn't download anything", func() {
			releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(primaryReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
			Expect(secondaryReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
		})
	})

	When("there's an error downloading the release", func() {
		var expectedError error

		BeforeEach(func() {
			expectedError = errors.New("boom")
			expectedRemoteRelease.SourceID = primaryReleaseSource.ID()
			primaryReleaseSource.GetMatchedReleaseReturns(*expectedRemoteRelease, true, nil)
			primaryReleaseSource.DownloadReleaseReturns(release.Local{}, expectedError)
		})

		It("returns that error", func() {
			_, _, err := releaseDownloader.DownloadRelease(downloadDir, requirement)
			Expect(err).To(MatchError(expectedError))
		})
	})
})
