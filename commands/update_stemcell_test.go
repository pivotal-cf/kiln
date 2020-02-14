package commands_test

import (
	"errors"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	test_helpers "github.com/pivotal-cf/kiln/internal/test-helpers"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/commands"
)

var _ = Describe("UpdateStemcell", func() {
	const (
		initialKilnfileYAMLFileContents = `---
`

		initialKilnfileLockFileContents = `---
stemcell_criteria:
  os: old-os
  version: "0.1"
`
		newStemcellOS      = "some-os"
		newStemcellVersion = "1.2.3"

		release1Name    = "release1"
		release1Version = "1"
		release2Name    = "release2"
		release2Version = "2"

		newRelease1SHA        = "new-sha1-1"
		newRelease1RemotePath = "new-remote-path-1"
		newRelease2SHA        = "new-sha1-2"
		newRelease2RemotePath = "new-remote-path-2"

		publishableReleaseSourceID   = "publishable"
		unpublishableReleaseSourceID = "test-only"

		releasesDirPath = "releases-dir"
	)

	Describe("Run", func() {
		var (
			update                                               *UpdateStemcell
			tmpDir, kilnfilePath, kilnfileLockPath, stemcellPath string
			kilnfileLock                                         cargo.KilnfileLock
			releaseSource                                        *fetcherFakes.ReleaseSource
			outputBuffer                                         *gbytes.Buffer
		)

		BeforeEach(func() {
			var err error

			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:         release1Name,
						SHA1:         "old-sha-1",
						Version:      release1Version,
						RemoteSource: "old-remote-source-1",
						RemotePath:   "old-remote-path-1",
					},
					{
						Name:         release2Name,
						SHA1:         "old-sha-2",
						Version:      release2Version,
						RemoteSource: "old-remote-source-2",
						RemotePath:   "old-remote-path-2",
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "old-os",
					Version: "0.1",
				},
			}

			releaseSource = new(fetcherFakes.ReleaseSource)
			releaseSource.GetMatchedReleaseCalls(func(requirement release.Requirement) (release.Remote, bool, error) {
				switch requirement.Name {
				case release1Name:
					remote := release.Remote{
						ID:         release.ID{Name: release1Name, Version: release1Version},
						RemotePath: newRelease1RemotePath,
						SourceID:   publishableReleaseSourceID,
					}
					return remote, true, nil
				case release2Name:
					remote := release.Remote{
						ID:         release.ID{Name: release2Name, Version: release2Version},
						RemotePath: newRelease2RemotePath,
						SourceID:   unpublishableReleaseSourceID,
					}
					return remote, true, nil
				default:
					panic("unexpected release name")
				}
			})

			releaseSource.DownloadReleaseCalls(func(_ string, remote release.Remote, _ int) (release.Local, error) {
				switch remote.Name {
				case release1Name:
					local := release.Local{
						ID:        release.ID{Name: release1Name, Version: release1Version},
						LocalPath: "not-used",
						SHA1:      newRelease1SHA,
					}
					return local, nil
				case release2Name:
					local := release.Local{
						ID:        release.ID{Name: release2Name, Version: release2Version},
						LocalPath: "not-used",
						SHA1:      newRelease2SHA,
					}
					return local, nil
				default:
					panic("unexpected release name")
				}
			})

			multiReleaseSourceProvider := new(fakes.MultiReleaseSourceProvider)
			multiReleaseSourceProvider.Returns(releaseSource)

			tmpDir, err = ioutil.TempDir("", "fetch-test")
			Expect(err).NotTo(HaveOccurred())

			kilnfilePath = filepath.Join(tmpDir, "Kilnfile")
			kilnfileLockPath = kilnfilePath + ".lock"

			stemcellPath = filepath.Join(tmpDir, "my-stemcell.tgz")
			test_helpers.WriteStemcellTarball(stemcellPath, newStemcellOS, newStemcellVersion, osfs.New(""))

			Expect(
				ioutil.WriteFile(kilnfilePath, []byte(initialKilnfileYAMLFileContents), 0644),
			).To(Succeed())

			Expect(
				ioutil.WriteFile(kilnfileLockPath, []byte(initialKilnfileLockFileContents), 0644),
			).To(Succeed())

			outputBuffer = gbytes.NewBuffer()
			logger := log.New(outputBuffer, "", 0)

			update = &UpdateStemcell{
				StemcellFile: stemcellPath,
				ReleasesDir: releasesDirPath,

				KilnfileLockPath: kilnfileLockPath,
				MultiReleaseSourceProvider: multiReleaseSourceProvider.Spy,
				Logger:                     logger,
			}
		})

		JustBeforeEach(func() {
			kilnfileLockFile, err := os.Create(kilnfileLockPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(
				yaml.NewEncoder(kilnfileLockFile).Encode(kilnfileLock),
			).To(Succeed())

			update.KilnfileLock = kilnfileLock
		})

		AfterEach(func() {
			Expect(
				os.RemoveAll(tmpDir),
			).To(Succeed())
		})

		It("updates the Kilnfile.lock contents", func() {
			err := update.Run(nil)
			Expect(err).NotTo(HaveOccurred())

			kilnfileLockFile, err := os.Open(kilnfileLockPath)
			Expect(err).NotTo(HaveOccurred())

			var actualKilnfileLock cargo.KilnfileLock
			err = yaml.NewDecoder(kilnfileLockFile).Decode(&actualKilnfileLock)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualKilnfileLock.Stemcell).To(Equal(cargo.Stemcell{
				OS:      newStemcellOS,
				Version: newStemcellVersion,
			}))

			Expect(actualKilnfileLock.Releases).To(Equal([]cargo.ReleaseLock{
				{
					Name:         release1Name,
					Version:      release1Version,
					SHA1:         newRelease1SHA,
					RemoteSource: publishableReleaseSourceID,
					RemotePath:   newRelease1RemotePath,
				},
				{
					Name:         release2Name,
					Version:      release2Version,
					SHA1:         newRelease2SHA,
					RemoteSource: unpublishableReleaseSourceID,
					RemotePath:   newRelease2RemotePath,
				},
			}))
		})

		It("looks up the correct releases", func() {
			err := update.Run(nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(2))

			req1 := releaseSource.GetMatchedReleaseArgsForCall(0)
			Expect(req1).To(Equal(release.Requirement{
				Name: release1Name, Version: release1Version,
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
			}))

			req2 := releaseSource.GetMatchedReleaseArgsForCall(1)
			Expect(req2).To(Equal(release.Requirement{
				Name: release2Name, Version: release2Version,
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
			}))
		})

		It("downloads the correct releases", func() {
			err := update.Run(nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(2))

			actualDir, remote1, threads := releaseSource.DownloadReleaseArgsForCall(0)
			Expect(actualDir).To(Equal(releasesDirPath))
			Expect(remote1).To(Equal(
				release.Remote{
					ID:         release.ID{Name: release1Name, Version: release1Version},
					RemotePath: newRelease1RemotePath,
					SourceID:   publishableReleaseSourceID,
				},
			))
			Expect(threads).To(Equal(0))

			actualDir, remote2, threads := releaseSource.DownloadReleaseArgsForCall(1)
			Expect(actualDir).To(Equal(releasesDirPath))
			Expect(remote2).To(Equal(
				release.Remote{
					ID:         release.ID{Name: release2Name, Version: release2Version},
					RemotePath: newRelease2RemotePath,
					SourceID:   unpublishableReleaseSourceID,
				},
			))
			Expect(threads).To(Equal(0))
		})

		When("the stemcell didn't change", func() {
			BeforeEach(func() {
				kilnfileLock.Stemcell = cargo.Stemcell{
					OS:      newStemcellOS,
					Version: newStemcellVersion,
				}
			})

			It("no-ops", func() {
				err := update.Run(nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(0))

				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(0))

				kilnfileLockFile, err := os.Open(kilnfileLockPath)
				Expect(err).NotTo(HaveOccurred())

				var actualKilnfileLock cargo.KilnfileLock
				err = yaml.NewDecoder(kilnfileLockFile).Decode(&actualKilnfileLock)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualKilnfileLock).To(Equal(kilnfileLock))
				Expect(outputBuffer.Contents()).To(ContainSubstring("Nothing to update for product"))
			})
		})

		When("the remote information for a release doesn't change", func() {
			BeforeEach(func() {
				kilnfileLock.Releases[1].RemoteSource = unpublishableReleaseSourceID
				kilnfileLock.Releases[1].RemotePath = newRelease2RemotePath
			})

			It("doesn't download the release", func() {
				err := update.Run(nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, remote, _ := releaseSource.DownloadReleaseArgsForCall(0)
				Expect(remote.Name).To(Equal(release1Name))

				Expect(string(outputBuffer.Contents())).To(ContainSubstring("No change"))
				Expect(string(outputBuffer.Contents())).To(ContainSubstring(release2Name))
			})
		})

		When("the release can't be found", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(release.Remote{}, false, nil)
			})

			It("errors", func() {
				err := update.Run(nil)

				Expect(err).To(MatchError(ContainSubstring("couldn't find release")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
			})
		})

		When("finding the release errors", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(release.Remote{}, false, errors.New("big badda boom"))
			})

			It("errors", func() {
				err := update.Run(nil)

				Expect(err).To(MatchError(ContainSubstring("finding release")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
				Expect(err).To(MatchError(ContainSubstring("big badda boom")))
			})
		})

		When("downloading the release errors", func() {
			BeforeEach(func() {
				releaseSource.DownloadReleaseReturns(release.Local{}, errors.New("big badda boom"))
			})

			It("errors", func() {
				err := update.Run(nil)

				Expect(err).To(MatchError(ContainSubstring("downloading release")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
				Expect(err).To(MatchError(ContainSubstring("big badda boom")))
			})
		})
	})
})
